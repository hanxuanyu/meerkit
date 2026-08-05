import { build } from "esbuild";
import { mkdir, readFile, writeFile } from "node:fs/promises";

await build({
  entryPoints: ["src/main.jsx"],
  bundle: true,
  minify: true,
  sourcemap: false,
  format: "iife",
  target: "es2020",
  outfile: "dist/assets/app.js",
  loader: { ".js": "jsx", ".jsx": "jsx", ".css": "css" },
  assetNames: "assets/[name]-[hash]"
});

await mkdir("dist", { recursive: true });
const index = await readFile("index.html", "utf8");
await writeFile("dist/index.html", index.replace('/src/main.jsx', '/assets/app.js'));
