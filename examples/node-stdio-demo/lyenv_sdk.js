// lyenv_sdk.js - Minimal Node SDK for lyenv stdio plugins.
// CommonJS for compatibility.

let _REQUEST = null;

const _RESPONSE = {
  status: "ok",
  logs: [],
  artifacts: [],
  mutations: { global: {}, plugin: {} },
};

function read_request() {
  // Read exactly one line JSON from stdin
  return new Promise((resolve, reject) => {
    let data = "";
    process.stdin.setEncoding("utf8");
    process.stdin.once("data", (chunk) => { data += chunk; });
    process.stdin.once("end", () => {
      const line = (data.split("\n")[0] || "").trim();
      if (!line) return reject(new Error("lyenv_sdk: empty stdin"));
      try {
        _REQUEST = JSON.parse(line);
        resolve(_REQUEST);
      } catch (e) {
        reject(e);
      }
    });
    // If stdin ends immediately, end event will fire.
  });
}

function _ensure_request_loaded() {
  if (!_REQUEST) throw new Error("lyenv_sdk: call read_request() first");
}

function log(msg) {
  _RESPONSE.logs.push(String(msg));
}

function emit_artifact(path) {
  _RESPONSE.artifacts.push(String(path));
}

function _set_by_path(obj, dotted, val) {
  const parts = String(dotted).split(".");
  let cur = obj;
  for (let i = 0; i < parts.length; i++) {
    const p = parts[i];
    if (i === parts.length - 1) cur[p] = val;
    else {
      if (!cur[p] || typeof cur[p] !== "object") cur[p] = {};
      cur = cur[p];
    }
  }
}

function plugin_write_config(key, value, scope = "plugin") {
  _ensure_request_loaded();
  const target = scope === "global" ? _RESPONSE.mutations.global : _RESPONSE.mutations.plugin;
  _set_by_path(target, key, value);
}

function respond_ok(message = "") {
  if (message) _RESPONSE.message = message;
  process.stdout.write(JSON.stringify(_RESPONSE) + "\n");
}

function respond_error(message) {
  _RESPONSE.status = "error";
  _RESPONSE.message = String(message);
  process.stdout.write(JSON.stringify(_RESPONSE) + "\n");
  process.exit(1);
}

module.exports = {
  read_request,
  log,
  emit_artifact,
  plugin_write_config,
  respond_ok,
  respond_error,
};
