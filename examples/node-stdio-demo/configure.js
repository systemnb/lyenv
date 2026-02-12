#!/usr/bin/env node
// Demo stdio command: read request, write one-line mutations via SDK.

const sdk = require("./lyenv_sdk.js");

async function main() {
  const req = await sdk.read_request();

  sdk.plugin_write_config("hello.enabled", true, "plugin");
  sdk.plugin_write_config("demo.last_run_at", new Date().toISOString(), "global");
  sdk.log(`os=${JSON.stringify(req.system || {})}`);

  sdk.respond_ok("ok");
}

main().catch((err) => {
  try {
    sdk.respond_error(err.message || String(err));
  } catch {
    process.stderr.write(String(err) + "\n");
    process.exit(1);
  }
});
