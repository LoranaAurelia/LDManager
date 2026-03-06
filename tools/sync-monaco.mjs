import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import https from "node:https";
import zlib from "node:zlib";

const projectRoot = process.cwd();
const targetRoot = path.join(projectRoot, "internal", "server", "static", "vendor", "monaco");
const tempRoot = fs.mkdtempSync(path.join(os.tmpdir(), "ldm-monaco-"));
const metaURL = "https://registry.npmjs.org/monaco-editor/latest";

function removeDir(dir) {
  fs.rmSync(dir, { recursive: true, force: true });
}

function copyDir(src, dst) {
  fs.mkdirSync(dst, { recursive: true });
  for (const entry of fs.readdirSync(src, { withFileTypes: true })) {
    const srcPath = path.join(src, entry.name);
    const dstPath = path.join(dst, entry.name);
    if (entry.isDirectory()) {
      copyDir(srcPath, dstPath);
      continue;
    }
    fs.copyFileSync(srcPath, dstPath);
  }
}

function fetchBuffer(url, redirects = 0) {
  return new Promise((resolve, reject) => {
    if (redirects > 5) {
      reject(new Error("too many redirects"));
      return;
    }
    const req = https.get(url, (res) => {
      const status = res.statusCode || 0;
      if (status >= 300 && status < 400 && res.headers.location) {
        res.resume();
        const next = new URL(res.headers.location, url).toString();
        fetchBuffer(next, redirects + 1).then(resolve, reject);
        return;
      }
      if (status < 200 || status >= 300) {
        res.resume();
        reject(new Error(`request failed: ${status}`));
        return;
      }
      const chunks = [];
      res.on("data", (chunk) => chunks.push(chunk));
      res.on("end", () => resolve(Buffer.concat(chunks)));
    });
    req.on("error", reject);
  });
}

async function fetchJSON(url) {
  const raw = await fetchBuffer(url);
  return JSON.parse(raw.toString("utf8"));
}

async function downloadFile(url, destination) {
  const data = await fetchBuffer(url);
  fs.writeFileSync(destination, data);
}

function readTarString(buf, start, length) {
  const raw = buf.subarray(start, start + length);
  const nul = raw.indexOf(0);
  return raw.subarray(0, nul >= 0 ? nul : raw.length).toString("utf8");
}

function readTarSize(buf, start, length) {
  const raw = readTarString(buf, start, length).trim();
  if (!raw) {
    return 0;
  }
  return Number.parseInt(raw, 8) || 0;
}

function extractTgz(tgzPath, destination) {
  const gz = fs.readFileSync(tgzPath);
  const tar = zlib.gunzipSync(gz);
  let offset = 0;

  while (offset + 512 <= tar.length) {
    const header = tar.subarray(offset, offset + 512);
    if (header.every((byte) => byte === 0)) {
      break;
    }

    const name = readTarString(header, 0, 100);
    const prefix = readTarString(header, 345, 155);
    const fullName = prefix ? `${prefix}/${name}` : name;
    const size = readTarSize(header, 124, 12);
    const typeFlag = readTarString(header, 156, 1) || "0";

    offset += 512;
    const dataEnd = offset + size;
    const data = tar.subarray(offset, dataEnd);

    const outPath = path.join(destination, ...fullName.split("/"));
    if (typeFlag === "5") {
      fs.mkdirSync(outPath, { recursive: true });
    } else if (typeFlag === "0") {
      fs.mkdirSync(path.dirname(outPath), { recursive: true });
      fs.writeFileSync(outPath, data);
    }

    offset = dataEnd;
    const remainder = offset % 512;
    if (remainder !== 0) {
      offset += 512 - remainder;
    }
  }
}

async function main() {
  const meta = await fetchJSON(metaURL);
  const tarball = meta?.dist?.tarball;
  if (!tarball) {
    throw new Error("monaco metadata does not include a tarball URL");
  }

  const packed = path.join(tempRoot, "monaco-editor.tgz");
  await downloadFile(tarball, packed);
  extractTgz(packed, tempRoot);

  const pkgDir = path.join(tempRoot, "package");
  const srcVS = path.join(pkgDir, "min", "vs");
  const pkgJSON = JSON.parse(fs.readFileSync(path.join(pkgDir, "package.json"), "utf8"));

  fs.mkdirSync(targetRoot, { recursive: true });
  copyDir(srcVS, path.join(targetRoot, "vs"));
  fs.writeFileSync(
    path.join(targetRoot, "version.json"),
    JSON.stringify(
      {
        package: "monaco-editor",
        version: pkgJSON.version || "unknown",
        synced_at: new Date().toISOString(),
        tarball,
      },
      null,
      2,
    ) + "\n",
    "utf8",
  );

  console.log(`Monaco synced: ${pkgJSON.version || "unknown"}`);
}

main().catch((error) => {
  const message = error instanceof Error ? error.message : String(error);
  console.error(`Monaco sync failed: ${message}`);
  process.exitCode = 1;
}).finally(() => {
  removeDir(tempRoot);
});
