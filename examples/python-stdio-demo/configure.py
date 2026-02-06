#!/usr/bin/env python3
# Demo stdio command: read request, write one-line mutations via SDK.
import time
from lyenv_sdk import read_request, log, plugin_write_config, respond_ok

def main():
    req = read_request()
    # one-liner: write plugin local config
    plugin_write_config("hello.enabled", True, scope="plugin")
    # write global config
    plugin_write_config("demo.last_run_at", time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), scope="global")
    log(f"os={req.get('system',{})}")
    respond_ok("ok")

if __name__ == "__main__":
    main()
