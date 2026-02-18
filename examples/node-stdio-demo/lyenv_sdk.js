// lyenv_sdk.js - Node.js SDK for lyenv stdio plugins (enhanced)
// - read request JSON from stdin
// - config_get (plugin/global), dotted paths
// - mutate + plugin_write_config alias
// - logs/artifacts + respond_ok/error

let REQUEST = null;
let RESPONDED = false;

const RESPONSE = {
  status: "ok",
  logs: [],
  artifacts: [],
  mutations: { global: {}, plugin: {} },
};

function readAllStdin() {
  return new Promise((resolve) => {
    let data = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("data", (chunk) => (data += chunk));
    process.stdin.on("end", () => resolve(data));
  });
}

async function read_request() {
  const raw = await readAllStdin();
  if (!raw || !raw.trim()) throw new Error("lyenv_sdk: empty stdin");
  REQUEST = JSON.parse(raw);
  return REQUEST;
}

function ensure_request_loaded() {
  if (!REQUEST) throw new Error("lyenv_sdk: call read_request() first");
}

function request() {
  ensure_request_loaded();
  return REQUEST;
}

function action() {
  ensure_request_loaded();
  return String(REQUEST.action || "");
}

function args() {
  ensure_request_loaded();
  return (REQUEST.args || []).map(String);
}

function dispatch_id() {
  ensure_request_loaded();
  return String(REQUEST.dispatch_id || REQUEST.dispatchId || "");
}

function paths() {
  ensure_request_loaded();
  return REQUEST.paths || {};
}

function system() {
  ensure_request_loaded();
  return REQUEST.system || {};
}

function get_path(name, defVal = "") {
  const p = paths();
  return name in p ? String(p[name]) : defVal;
}

function get_by_path(obj, dotted, defVal = undefined) {
  if (!dotted) return obj;
  let cur = obj;
  for (const p of dotted.split(".")) {
    if (cur && typeof cur === "object" && p in cur) cur = cur[p];
    else return defVal;
  }
  return cur;
}

function config_scope(scope = "plugin") {
  ensure_request_loaded();
  const cfg = REQUEST.config || {};
  return scope === "global" ? (cfg.global || {}) : (cfg.plugin || {});
}

function config_get(key, defVal = undefined, scope = "plugin") {
  return get_by_path(config_scope(scope), key, defVal);
}

function config_plugin(key, defVal = undefined) {
  return config_get(key, defVal, "plugin");
}

function config_global(key, defVal = undefined) {
  return config_get(key, defVal, "global");
}

function log(msg) {
  RESPONSE.logs.push(String(msg));
}

function log_stream(msg) {
  const s = String(msg);
  try {
    process.stderr.write(s + "\n"); // real-time CLI
  } catch {}
  // also keep it in final response logs
  RESPONSE.logs.push(s);
}


function emit_artifact(p) {
  RESPONSE.artifacts.push(String(p));
}

function set_by_path(target, dotted, value) {
  if (!dotted) throw new Error("empty dotted key");
  const parts = dotted.split(".");
  let cur = target;
  for (let i = 0; i < parts.length; i++) {
    const k = parts[i];
    if (i === parts.length - 1) cur[k] = value;
    else {
      if (!(k in cur) || typeof cur[k] !== "object") cur[k] = {};
      cur = cur[k];
    }
  }
}

function mutate(key, value, scope = "plugin") {
  ensure_request_loaded();
  const tgt = scope === "global" ? RESPONSE.mutations.global : RESPONSE.mutations.plugin;
  set_by_path(tgt, key, value);
}

// Backward/forward compatible alias (like Python)
function plugin_write_config(key, value, scope = "plugin", merge = null) {
  // merge is reserved; core handles merge strategy
  mutate(key, value, scope);
}

function global_write_config(key, value) {
  mutate(key, value, "global");
}

function ensure_not_responded() {
  if (RESPONDED) throw new Error("lyenv_sdk: respond_* called more than once");
  RESPONDED = true;
}

function respond_ok(message = "", extra = null) {
  ensure_not_responded();
  if (message && String(message).trim()) RESPONSE.message = String(message);
  if (extra && typeof extra === "object") Object.assign(RESPONSE, extra);
  process.stdout.write(JSON.stringify(RESPONSE) + "\n");
}

function respond_error(message, code = 1, extra = null) {
  ensure_not_responded();
  RESPONSE.status = "error";
  RESPONSE.message = String(message);
  if (extra && typeof extra === "object") Object.assign(RESPONSE, extra);
  process.stdout.write(JSON.stringify(RESPONSE) + "\n");
  process.exit(code);
}

module.exports = {
  // request
  read_request, request, action, args, dispatch_id, paths, system, get_path,
  // config
  config_get, config_plugin, config_global,
  // response
  log, log_stream, emit_artifact, mutate, plugin_write_config, global_write_config,
  respond_ok, respond_error,
};
