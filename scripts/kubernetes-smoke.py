#!/usr/bin/env python3
"""Check a Helm deployment through its port-forward, including PVC restart recovery."""
import base64
import http.client
import json
import os
from pathlib import Path
import subprocess
import time
import urllib.error
import urllib.request

base = os.environ.get('PANL_URL','http://127.0.0.1:8080')
ns = os.environ.get('PANL_NAMESPACE','panl')
release = os.environ.get('PANL_RELEASE','panl')
kubectl = ['kubectl']
if os.environ.get('KUBECONFIG'):
    kubectl += ['--kubeconfig',os.environ['KUBECONFIG']]

def command(*args):
    return subprocess.check_output(kubectl+['-n',ns,*args],text=True)
def wait_ready():
    for _ in range(40):
        try:
            with urllib.request.urlopen(base+'/readyz',timeout=2) as r:
                if r.status==200:return
        except (urllib.error.URLError,TimeoutError,http.client.HTTPException,ConnectionError):pass
        time.sleep(.5)
    raise RuntimeError('deployment did not become reachable')

def request(path,body=None,method='POST',token=None):
    headers={}
    if body is not None:headers['Content-Type']='application/json'
    if token:headers['Authorization']='Bearer '+token
    req=urllib.request.Request(base+path,data=None if body is None else json.dumps(body).encode(),headers=headers,method='GET' if body is None else method)
    with urllib.request.urlopen(req,timeout=30) as r:return json.load(r)

wait_ready()
encoded=command('get','secret',release+'-agent','-o','jsonpath={.data.token}')
token=base64.b64decode(encoded).decode()
if not os.environ.get('PANL_SMOKE_ALLOW_EXISTING'):
    assert request('/api/dashboards')==[], 'smoke test requires seed=none and a fresh PVC'
try:
    request('/api/dashboards',{'id':'unauthorized'})
except urllib.error.HTTPError as e:
    assert e.code==401
else:raise AssertionError('unauthenticated configuration was accepted')
dashboard={'id':'smoke-'+str(int(time.time())),'title':'Agent-created','description':'Kubernetes persistence verification','revision':0,'panels':[{'id':'cpu','title':'CPU usage','unit':'cores','queries':[{'expr':'sum by(namespace)(rate(container_cpu_usage_seconds_total{namespace=~"$namespace"}[5m]))','legend':'{{namespace}}'}]}]}
saved=request('/api/dashboards',dashboard,token=token)
assert saved['revision']==1
sample=request('/api/query',{'expr':dashboard['panels'][0]['queries'][0]['expr'],'namespace':'','start':int(time.time())-3600,'end':int(time.time()),'points':100})
assert sample['series'] and any(p['value'] is not None for s in sample['series'] for p in s['points'])
command('rollout','restart','deployment/'+release)
command('rollout','status','deployment/'+release,'--timeout=120s')
# The caller's port-forward may need restarting after a pod replacement.
print('Pod restarted; reconnecting the local port-forward')
forward=subprocess.Popen(kubectl+['-n',ns,'port-forward','svc/'+release,base.rsplit(':',1)[-1]+':8080'],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
try:
    wait_ready()
    restored=request('/api/dashboards/'+dashboard['id'])
    assert restored['revision']==saved['revision'] and restored['panels']==saved['panels']
    assert command('get','secret',release+'-agent','-o','jsonpath={.data.token}')==encoded
    print('PASS: deployed chart, unauthorized writes rejected, real PromQL samples, PVC survives pod restart, agent credential preserved')
finally:forward.terminate();forward.wait(timeout=10)
