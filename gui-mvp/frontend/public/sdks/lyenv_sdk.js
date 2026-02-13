// lyenv_sdk.js - Node.js SDK for lyenv stdio plugins
// Features: read request, read config, write mutations, logs/artifacts, respond_ok/error.

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
  if (!raw || !raw.trim()) {
    throw new Error("lyenv_sdk: empty stdin");
  }
  REQUEST = JSON.parse(raw);
  return REQUEST;
}

function ensure_request_loaded() {
  if (!REQUEST) throw new Error("lyenv_sdk: call read_request() first");
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

function action() {
  ensure_request_loaded();
  return String(REQUEST.action || "");
}

function args() {
  ensure_request_loaded();
  return (REQUEST.args || []).map(String);
}

function paths() {
  ensure_request_loaded();
  return REQUEST.paths || {};
}

function get_path(name, defVal = "") {
  const p = paths();
  return name in p ? String(p[name]) : defVal;
}

function log(msg) {
  RESPONSE.logs.push(String(msg));
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
    if (i === parts.length - 1) {
      cur[k] = value;
    } else {
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

function respond_ok(message = "", extra = null) {
  if (RESPONDED) throw new Error("lyenv_sdk: respond_* called more than once");
  RESPONDED = true;
  if (message && String(message).trim()) RESPONSE.message = String(message);
  if (extra && typeof extra === "object") Object.assign(RESPONSE, extra);
  process.stdout.write(JSON.stringify(RESPONSE) + "\n");
}

function respond_error(message, code = 1, extra = null) {
  if (RESPONDED) throw new Error("lyenv_sdk: respond_* called more than once");
  RESPONDED = true;
  RESPONSE.status = "error";
  RESPONSE.message = String(message);
  if (extra && typeof extra === "object") Object.assign(RESPONSE, extra);
  process.stdout.write(JSON.stringify(RESPONSE) + "\n");
  process.exit(code);
}

module.exports = {
  read_request,
  action,
  args,
  paths,
  get_path,
  config_get,
  log,
  emit_artifact,
  mutate,
  respond_ok,
  respond_error,
};
