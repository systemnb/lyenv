// Minimal Node SDK for lyenv stdio plugins
const readline = require('readline');

const RESPONSE = {
  status: 'ok',
  logs: [],
  artifacts: [],
  mutations: { global: {}, plugin: {} }
};

function readRequest() {
  const rl = readline.createInterface({ input: process.stdin });
  return new Promise((resolve) => {
    rl.on('line', (line) => {
      rl.close();
      resolve(JSON.parse(line));
    });
  });
}

function log(msg) { RESPONSE.logs.push(String(msg)); }
function emitArtifact(p) { RESPONSE.artifacts.push(String(p)); }

function pluginWriteConfig(key, value, scope = 'plugin') {
  const target = scope === 'global' ? RESPONSE.mutations.global : RESPONSE.mutations.plugin;
  const parts = key.split('.');
  let cur = target;
  parts.forEach((p, i) => {
    if (i === parts.length - 1) cur[p] = value;
    else cur = (cur[p] = cur[p] || {});
  });
}

function respondOk(message = '') {
  if (message) RESPONSE.message = message;
  process.stdout.write(JSON.stringify(RESPONSE) + '\n');
}

function respondError(message) {
  RESPONSE.status = 'error';
  RESPONSE.message = message || 'error';
  process.stdout.write(JSON.stringify(RESPONSE) + '\n');
  process.exit(1);
}

module.exports = {
  readRequest,
  log,
  emitArtifact,
  pluginWriteConfig,
  respondOk,
  respondError,
};
