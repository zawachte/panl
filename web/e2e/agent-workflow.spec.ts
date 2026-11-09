import {test,expect} from '@playwright/test';
test.use({baseURL:'http://127.0.0.1:18085',video:'on'});
test('an authenticated agent builds and refines an empty dashboard using MCP only',async({page,request})=>{
 let requestId=0;
 async function rpc(method:string,params:unknown){const r=await request.post('/mcp',{headers:{Authorization:'Bearer playwright-agent-token-for-local-tests',Accept:'application/json, text/event-stream','MCP-Protocol-Version':'2025-06-18'},data:{jsonrpc:'2.0',id:++requestId,method,params}});expect(r.ok()).toBeTruthy();const data=await r.json();expect(data.error).toBeUndefined();expect(data.result?.isError).not.toBe(true);return data.result}
 async function tool(name:string,args:unknown){return (await rpc('tools/call',{name,arguments:args})).structuredContent}
 await rpc('initialize',{protocolVersion:'2025-06-18',capabilities:{},clientInfo:{name:'release-walkthrough',version:'1'}});
 expect((await tool('list_dashboards',{})).dashboards).toEqual([]);
 const info=await tool('get_server_info',{});expect(info.mode).toBe('demo');
 expect((await tool('discover_metrics',{prefix:'container_cpu'})).values).toContain('container_cpu_usage_seconds_total');
 await page.goto('/');await expect(page.getByRole('heading',{name:'No dashboards yet'})).toBeVisible();
 let dashboard=await tool('create_dashboard',{dashboard:{id:'namespace-resources',title:'Namespace resources',description:'Built through MCP',revision:0,panels:[]}});
 await page.goto('/?dashboard=namespace-resources');await expect(page.getByRole('heading',{name:'This dashboard is empty'})).toBeVisible();
 let cpu={id:'cpu',title:'CPU usage',unit:'cores',queries:[{expr:'sum by(namespace)(rate(container_cpu_usage_seconds_total{namespace=~"$namespace",container!="",image!=""}[5m]))',legend:'{{namespace}}',kind:''}]};
 expect((await tool('validate_dashboard',{dashboard:{...dashboard,panels:[cpu]}})).valid).toBe(true);
 dashboard=await tool('add_panel',{dashboard_id:dashboard.id,revision:dashboard.revision,panel:cpu});
 await expect(page.getByRole('heading',{name:'CPU usage',exact:true})).toBeVisible({timeout:10000});await expect(page.locator('canvas')).toHaveCount(1);
 cpu={...cpu,title:'CPU by pod',queries:[{...cpu.queries[0],expr:cpu.queries[0].expr.replace('by(namespace)','by(namespace,pod)'),legend:'{{pod}}'}]};
 dashboard=await tool('update_panel',{dashboard_id:dashboard.id,revision:dashboard.revision,panel:cpu});
 await expect(page.getByRole('heading',{name:'CPU by pod',exact:true})).toBeVisible({timeout:10000});
 cpu={...cpu,title:'CPU usage, requests & limits',queries:[...cpu.queries,...(['request','limit'] as const).map(kind=>({expr:`sum by(namespace,pod)(kube_pod_container_resource_${kind==='request'?'requests':'limits'}{namespace=~"$namespace",resource="cpu"})`,legend:`${kind} · {{pod}}`,kind}))]};
 const checked=await tool('validate_dashboard',{dashboard:{...dashboard,panels:[cpu]}});expect(checked.valid).toBe(true);expect(checked.queries[2].series).toBeLessThan(checked.queries[0].series);
 dashboard=await tool('update_panel',{dashboard_id:dashboard.id,revision:dashboard.revision,panel:cpu});
 await expect(page.getByRole('heading',{name:cpu.title,exact:true})).toBeVisible({timeout:10000});
 const preservedCPU=structuredClone(dashboard.panels[0]);
 const memory={id:'memory',title:'Memory by namespace',unit:'bytes',queries:[{expr:'sum by(namespace)(container_memory_working_set_bytes{namespace=~"$namespace",container!="",image!=""})',legend:'{{namespace}}'}]};
 dashboard=await tool('add_panel',{dashboard_id:dashboard.id,revision:dashboard.revision,panel:memory});
 expect(dashboard.panels[0]).toEqual(preservedCPU);
 await expect(page.getByRole('heading',{name:memory.title,exact:true})).toBeVisible({timeout:10000});await expect(page.locator('canvas')).toHaveCount(2);
 await page.selectOption('#namespace','checkout');await expect(page.locator('.legend-row').first()).toContainText('checkout');
 await expect(page.locator('input,textarea,[contenteditable="true"]')).toHaveCount(0);
 await page.screenshot({path:'/tmp/panl-agent-workflow.png',fullPage:true});
 const video=page.video();await page.context().close();await video?.saveAs('/tmp/panl-agent-workflow.webm');
});
