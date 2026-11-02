#!/usr/bin/env node
// Runs the shj binary from the package for this OS and CPU, which npm installs
// as one of the optionalDependencies (npm skips the ones whose os and cpu do not match).
"use strict";

const { spawnSync } = require("node:child_process");

const pkg = `shijimi-editor-${process.platform}-${process.arch}`;
let bin;
try {
  bin = require.resolve(`${pkg}/bin/shj`);
} catch {
  console.error(
    `shijimi-editor: ${pkg} is not installed. ` +
      "Builds exist for Linux and macOS on x64 and arm64. " +
      "If you installed with --no-optional (or --omit=optional), reinstall without it.",
  );
  process.exit(1);
}

const r = spawnSync(bin, process.argv.slice(2), { stdio: "inherit" });
if (r.error) throw r.error;
// Exit the same way the binary did, so that a shell sees the same signal or status.
if (r.signal) process.kill(process.pid, r.signal);
process.exit(r.status ?? 1);
