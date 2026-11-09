import {randomUUID} from "node:crypto";
import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./e2e",
  workers: 1,
  timeout: 45000,
  use: {
    baseURL: "http://127.0.0.1:18080",
    viewport: { width: 1440, height: 1040 },
    trace: "retain-on-failure",
  },
  webServer: [{
    command:
      "../panl --serve --listen 127.0.0.1:18080 --db /tmp/panl-e2e-dashboard.db --web-dist dist --demo",
    url: "http://127.0.0.1:18080",
    reuseExistingServer: false,
    env: { PANL_AGENT_TOKEN: "playwright-agent-token-for-local-tests" },
  },{
 command:`../panl --serve --listen 127.0.0.1:18085 --db /tmp/panl-agent-e2e-${randomUUID()}.db --web-dist dist --demo --seed none`,
 url:"http://127.0.0.1:18085",reuseExistingServer:false,env:{PANL_AGENT_TOKEN:"playwright-agent-token-for-local-tests"}
 }],
});
