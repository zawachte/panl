// Run from web/ with: node ../scripts/capture.mjs <URL> <screenshot-path>
import {createRequire} from 'node:module';
import {resolve} from 'node:path';
const require=createRequire(resolve(process.cwd(),'package.json'));
const {chromium}=require('@playwright/test');
const url=process.argv[2]||'http://127.0.0.1:8080';
const path=process.argv[3]||'/tmp/panl.png';
const browser=await chromium.launch({headless:true});
const page=await browser.newPage({viewport:{width:1440,height:1000}});
const errors=[];const external=[];
page.on('pageerror',e=>errors.push(e.message));
page.on('request',r=>{if(new URL(r.url()).origin!==new URL(url).origin)external.push(r.url())});
await page.goto(url);
await page.waitForFunction(()=>document.querySelectorAll('canvas').length>0,{timeout:30000});
await page.evaluate(()=>document.fonts.ready);
await page.screenshot({path,fullPage:true});
console.log(JSON.stringify({screenshot:path,charts:await page.locator('canvas').count(),errors,externalRequests:external}));
await browser.close();
if(errors.length||external.length)process.exitCode=1;
