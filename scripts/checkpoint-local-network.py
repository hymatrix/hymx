#!/usr/bin/env python3
"""Local integration: core VM actions, stop checkpoints and recovery.
Run from the repository: python3 scripts/checkpoint-local-network.py
Creates temporary artifacts and cleans up only its own subprocesses. No public
network, chainkit or payment is enabled. Requires Go and redis-server.
"""
import base64
import json
import pathlib
import re
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.request

ROOT = pathlib.Path(__file__).resolve().parents[1]


def port():
    with socket.socket() as s:
        s.bind(('127.0.0.1', 0))
        return s.getsockname()[1]


def get(url):
    with urllib.request.urlopen(url, timeout=1) as r:
        return json.load(r)


def wait_for(fn, description, timeout=20):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        try:
            value = fn()
            if value:
                return value
        except (OSError, ValueError):
            pass
        time.sleep(.05)
    raise RuntimeError('timeout: ' + description)


def main():
    work = pathlib.Path(tempfile.mkdtemp(prefix='hymx-checkpoint-network-'))
    print('artifacts:', work, flush=True)
    processes = []
    handles = []
    summary = {}
    try:
        binary = work / 'hymx'
        subprocess.run(['go', 'build', '-o', str(binary), './cmd'], cwd=ROOT, check=True)
        rp, api, admin = port(), port(), port()
        rlog = open(work / 'redis.log', 'w'); handles.append(rlog)
        redis = subprocess.Popen(['redis-server', '--bind', '127.0.0.1', '--port', str(rp),
            '--save', '', '--appendonly', 'no', '--dir', str(work)], stdout=rlog, stderr=rlog)
        processes.append(redis)
        def redis_ready():
            with socket.create_connection(('127.0.0.1', rp), timeout=1) as s:
                s.sendall(b'*1\r\n$4\r\nPING\r\n')
                return s.recv(64).startswith(b'+PONG')
        wait_for(redis_ready, 'Redis ready')
        base = f'http://127.0.0.1:{api}'
        adm = f'http://127.0.0.1:{admin}'
        cfg = work / 'config.yaml'
        cfg.write_text('\n'.join([
            f'port: 127.0.0.1:{api}', f'adminPort: 127.0.0.1:{admin}', 'ginMode: release',
            f'redisURL: redis://127.0.0.1:{rp}/0', f'arweaveURL: {base}', f'hymxURL: {base}',
            f'keyfilePath: {ROOT / "cmd/test_keyfile.json"}', 'nodeName: p0-local',
            f'nodeURL: {base}', 'joinNetwork: false', 'enablePayment: false', 'enableChainkit: false',
        ]) + '\n')
        shutil.copytree(ROOT / 'cmd/mod', work / 'mod')
        version = re.search(r'NodeVersion\s*=\s*"([^"]+)"',
            (ROOT / 'node/schema/default.go').read_text()).group(1)
        # Current daemon lock naming; run as a directly owned child so exit can
        # be measured precisely, then exercise the actual stop subcommand.
        protocol = re.search(r'DataProtocol\s*=\s*"([^"]+)"',
            (ROOT / 'schema/schema.go').read_text()).group(1)
        lock = work / f'{protocol}-{version}.lock'
        def start(label):
            log = open(work / f'{label}.log', 'w'); handles.append(log)
            p = subprocess.Popen([str(binary), '--config', str(cfg)], cwd=work, stdout=log, stderr=log)
            processes.append(p)
            wait_for(lambda: get(base + '/info'), label + ' API')
            lock.write_text(str(p.pid))
            return p
        node = start('initial')
        helper = work / 'seed.go'
        helper.write_text('''package main
import (
 "encoding/json"
 "fmt"
 "os"
 "github.com/hymatrix/hymx/sdk"
 vs "github.com/hymatrix/hymx/vmm/schema"
 gs "github.com/permadao/goar/schema"
)
func main() {
 s:=sdk.New(os.Args[1],os.Args[2]); defer s.Close()
 t,e:=s.SpawnAndWait("1i03Vpe8DljkUMBEEEvR0VmbJjvgZtP_ytZdThkVSMw",s.GetAddress(),nil); if e!=nil {panic(e)}
 r,e:=s.SpawnAndWait("MVTil0kn5SRiJELW7W2jLZ6cBr3QUGj1nJ67I2Wi4Ps",s.GetAddress(),[]gs.Tag{{Name:"Token-Pid",Value:t.Id},{Name:"Name",Value:"p0-local"},{Name:"URL",Value:os.Args[1]}}); if e!=nil {panic(e)}
 for _, response := range []string{t.Message,r.Message} {
  var result vs.VmmResult
  if e=json.Unmarshal([]byte(response),&result); e!=nil {panic(e)}
  if result.Error!="" {panic(result.Error)}
 }
 send := func(pid, action string, tags ...gs.Tag) vs.VmmResult {
  response,err:=s.SendMessageAndWait(pid,"",append([]gs.Tag{{Name:"Action",Value:action}},tags...)); if err!=nil {panic(err)}
  var result vs.VmmResult
  if err=json.Unmarshal([]byte(response.Message),&result); err!=nil {panic(err)}
  if result.Error!="" {panic(result.Error)}
  fmt.Println("action passed",action,response.Id)
  return result
 }
 for _, action:=range []string{"Info","Balance","Total-Supply"} {
  if len(send(t.Id,action).Messages)==0 {panic("missing token response: "+action)}
 }
 recipient:="AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
 send(t.Id,"Transfer",gs.Tag{Name:"Recipient",Value:recipient},gs.Tag{Name:"Quantity",Value:"123"})
 send(t.Id,"Stake",gs.Tag{Name:"Registry",Value:r.Id},gs.Tag{Name:"Quantity",Value:"1000"},gs.Tag{Name:"Acc-Id",Value:s.GetAddress()},gs.Tag{Name:"Name",Value:"core-vm-verified"},gs.Tag{Name:"Desc",Value:"local integration"},gs.Tag{Name:"URL",Value:os.Args[1]})
 data,err:=json.Marshal(map[string]string{"account":s.GetAddress(),"recipient":recipient,"token":t.Id,"registry":r.Id}); if err!=nil {panic(err)}
 if err=os.WriteFile(os.Args[3],data,0600); err!=nil {panic(err)}
 fmt.Println("seeded",t.Id,r.Id)
}
''')
        with open(work / 'seed.log', 'w') as out:
            subprocess.run(['go', 'run', str(helper), base, str(ROOT / 'cmd/test_keyfile.json'), str(work/'seed.json')],
                           cwd=ROOT, stdout=out, stderr=out, check=True, timeout=60)
        pids = wait_for(lambda: (v if len(v:=get(adm+'/admin/vms/running'))==2 else None), 'two VMs')
        summary['vm_count'] = len(pids)
        summary['pids'] = sorted(pids)
        seed = json.loads((work/'seed.json').read_text())
        def core_state():
            nodes = get(base + '/nodes')
            assert nodes[seed['account']]['Name'] == 'core-vm-verified', nodes
            balances = {account: int(get(base+'/balanceOf/'+account))
                        for account in (seed['account'], seed['recipient'])}
            assert balances[seed['account']] == 20000000000000000000 - 123 - 1000, balances
            assert balances[seed['recipient']] == 123, balances
            stake = int(get(base+'/stakeOf/'+seed['account']))
            assert stake == 1000000000000000000 + 1000, stake
            registered = get(base+'/processes/'+seed['account'])
            assert seed['token'] in registered and seed['registry'] in registered, registered
            return {'nodes': nodes, 'balances': balances, 'stake': stake,
                    'registered_processes': sorted(registered)}
        def registry_updated():
            nodes = get(base+'/nodes')
            return nodes and nodes.get(seed['account'], {}).get('Name') == 'core-vm-verified'
        wait_for(registry_updated, 'Token Stake delivered Registry Register')
        summary['core_state_before'] = core_state()
        nodes_before = summary['core_state_before']['nodes']
        assert not list((work/'ckp').glob('ckp-*.json'))
        def stop(p, label, checkpoint=False):
            began = time.monotonic()
            args = [str(binary), 'stop'] + (['--checkpoint'] if checkpoint else [])
            cmd = subprocess.run(args, cwd=work, capture_output=True, text=True, timeout=10)
            returned = time.monotonic()
            (work/f'{label}-stop.log').write_text(cmd.stdout+cmd.stderr)
            alive = p.poll() is None
            rc = p.wait(timeout=30)
            finished = time.monotonic()
            assert cmd.returncode == 0 and rc == 0
            assert not lock.exists()
            return {'command_ms':round((returned-began)*1000,3),
                    'exit_ms':round((finished-began)*1000,3), 'alive_when_command_returned':alive}
        summary['default_stop'] = stop(node, 'initial')
        assert not list((work/'ckp').glob('ckp-*.json')), 'default stop saved VM checkpoints'
        node = start('history-recovery')
        wait_for(registry_updated, 'Registry restored from history without checkpoint')
        # Registry is the last core VM to receive the stake notification.
        assert core_state() == summary['core_state_before']
        summary['history_recovery'] = True
        summary['checkpoint_stop'] = stop(node, 'history-recovery', checkpoint=True)
        files = list((work/'ckp').glob('ckp-*.json'))
        assert len(files)==2, f'expected 2 checkpoints, got {len(files)}'
        summary['checkpoint_files'] = len(files)
        summary['checkpoint_bytes'] = sum(f.stat().st_size for f in files)
        checkpoint_contents = {f.name: f.read_bytes() for f in files}
        snapshots = []
        for f in files:
            item = json.loads(f.read_text())
            data = item.get('data', item.get('Data'))
            snap = json.loads(base64.urlsafe_b64decode(data + '=' * (-len(data) % 4)))
            snapshots.append(snap)
            assert snap['Data']
        assert sorted(s['Env']['Meta']['Pid'] for s in snapshots) == sorted(pids)
        summary['snapshot_nonces'] = {s['Env']['Meta']['Pid']: s['Env']['Nonce'] for s in snapshots}
        node = start('recovery')
        recovered = wait_for(lambda: (v if sorted(v:=get(adm+'/admin/vms/running'))==sorted(pids) else None), 'restored VMs')
        summary['recovered_pids'] = sorted(recovered)
        wait_for(lambda: get(base + '/nodes') == nodes_before, 'restored registry state')
        summary['registry_state_restored'] = True
        summary['core_state_after'] = core_state()
        assert summary['core_state_after'] == summary['core_state_before']
        summary['second_stop'] = stop(node, 'recovery')
        assert {f.name: f.read_bytes() for f in (work/'ckp').glob('ckp-*.json')} == checkpoint_contents
        print(json.dumps(summary, indent=2), flush=True)
        (work/'summary.json').write_text(json.dumps(summary,indent=2)+'\n')
    finally:
        for p in reversed(processes):
            if p.poll() is None:
                p.terminate()
                try: p.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    p.kill(); p.wait()
        for h in handles: h.close()

if __name__ == '__main__':
    main()
