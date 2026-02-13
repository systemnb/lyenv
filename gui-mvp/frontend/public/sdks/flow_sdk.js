// flow_sdk.js - Flow helper for lyenv stdio plugins (Node.js)
//
// Conventions:
// - Store outputs in plugin config:
//     flow.outputs.<node_id>.<port> = "<string>"
// - Wiring map:
//     wiring[dstNodeId][dstInputPort] = { node: srcNodeId, port: srcOutputPortName }
//
// Requires:
// - scripts/lyenv_sdk.js injected and require-able

const fs = require("fs");
const path = require("path");
const sdk = require("./lyenv_sdk.js");

function _flowKey(nodeId, port) {
  return `flow.outputs.${nodeId}.${port}`;
}

function loadWiring(filePath) {
  const p = String(filePath || "");
  const txt = fs.readFileSync(p, "utf8");
  return JSON.parse(txt || "{}");
}

function resolveRef(wiring, dstNodeId, dstInputPort) {
  const m = (wiring && wiring[dstNodeId]) || {};
  const ref = m[dstInputPort];
  if (ref && typeof ref === "object") {
    const srcNode = String(ref.node || "");
    const srcPort = String(ref.port || "");
    if (srcNode && srcPort) return [srcNode, srcPort];
  }
  return null;
}

function getOutput(req, nodeId, port, defVal = "") {
  // Prefer reading from request merged plugin config
  // sdk.config_plugin can read dotted paths from req.config.plugin
  try {
    const v = sdk.config_plugin(_flowKey(nodeId, port), defVal);
    return v == null ? "" : String(v);
  } catch {
    return defVal;
  }
}

function setOutput(nodeId, port, value) {
  // Write via plugin mutation
  sdk.plugin_write_config(_flowKey(nodeId, port), value == null ? "" : String(value), "plugin");
}

function setOutputs(nodeId, outputs) {
  const o = outputs || {};
  for (const k of Object.keys(o)) {
    setOutput(nodeId, k, o[k]);
  }
}

function getInputs(req, wiring, nodeId, inputPorts, defVal = "") {
  const ins = [];
  for (const name of (inputPorts || [])) {
    const ref = resolveRef(wiring, nodeId, name);
    if (ref) {
      const [srcNode, srcPort] = ref;
      ins.push(getOutput(req, srcNode, srcPort, defVal));
    } else {
      ins.push(defVal);
    }
  }
  return ins;
}

function getInput(req, wiring, nodeId, portName, defVal = "") {
  return getInputs(req, wiring, nodeId, [portName], defVal)[0] || defVal;
}

function debugDumpWiring(wiring, nodeId = null) {
  if (nodeId) sdk.log({ wiring: { [nodeId]: (wiring && wiring[nodeId]) || {} } });
  else sdk.log({ wiring: wiring || {} });
}

function debugDumpIO(req, wiring, nodeId, inputPorts, outputPorts = null) {
  const ins = getInputs(req, wiring, nodeId, inputPorts || [], "");
  sdk.log({ node: nodeId, inputs: Object.fromEntries((inputPorts || []).map((p, i) => [p, ins[i] ?? ""])) });

  if (outputPorts && outputPorts.length) {
    const outs = {};
    for (const p of outputPorts) outs[p] = getOutput(req, nodeId, p, "");
    sdk.log({ node: nodeId, outputs: outs });
  }
}

module.exports = {
  loadWiring,
  resolveRef,
  getOutput,
  setOutput,
  setOutputs,
  getInputs,
  getInput,
  debugDumpWiring,
  debugDumpIO,
};
