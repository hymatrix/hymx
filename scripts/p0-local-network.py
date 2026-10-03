#!/usr/bin/env python3
"""P0 characterization: isolated Redis + real node, stop checkpoints and recovery.
Run from the repository: python3 scripts/p0-local-network.py
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
    work = pathlib.Path(tempfile.mkdtemp(prefix='hymx-p0-network-'))
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
 "fmt"
 "os"
 "github.com/hymatrix/hymx/sdk"
 gs "github.com/permadao/goar/schema"
)
func main() {
 s:=sdk.New(os.Args[1],os.Args[2]); defer s.Close()
 t,e:=s.SpawnAndWait("1i03Vpe8DljkUMBEEEvR0VmbJjvgZtP_ytZdThkVSMw",s.GetAddress(),nil); if e!=nil {panic(e)}
 r,e:=s.SpawnAndWait("MVTil0kn5SRiJELW7W2jLZ6cBr3QUGj1nJ67I2Wi4Ps",s.GetAddress(),[]gs.Tag{{Name:"Token-Pid",Value:t.Id},{Name:"Name",Value:"p0-local"},{Name:"URL",Value:os.Args[1]}}); if e!=nil {panic(e)}
 fmt.Println("seeded",t.Id,r.Id)
}
''')
        with open(work / 'seed.log', 'w') as out:
            subprocess.run(['go', 'run', str(helper), base, str(ROOT / 'cmd/test_keyfile.json')],
                           cwd=ROOT, stdout=out, stderr=out, check=True, timeout=60)
        pids = wait_for(lambda: (v if len(v:=get(adm+'/admin/vms/running'))==2 else None), 'two VMs')
        summary['vm_count'] = len(pids)
        summary['pids'] = sorted(pids)
        nodes_before = get(base + '/nodes')
        assert not list((work/'ckp').glob('ckp-*.json'))
        def stop(p, label):
            began = time.monotonic()
            cmd = subprocess.run([str(binary),'stop'], cwd=work, capture_output=True, text=True, timeout=10)
            returned = time.monotonic()
            (work/f'{label}-stop.log').write_text(cmd.stdout+cmd.stderr)
            alive = p.poll() is None
            rc = p.wait(timeout=30)
            finished = time.monotonic()
            assert cmd.returncode == 0 and rc == 0
            assert not lock.exists()
            return {'command_ms':round((returned-began)*1000,3),
                    'exit_ms':round((finished-began)*1000,3), 'alive_when_command_returned':alive}
        summary['first_stop'] = stop(node, 'initial')
        files = list((work/'ckp').glob('ckp-*.json'))
        assert len(files)==2, f'expected 2 checkpoints, got {len(files)}'
        summary['checkpoint_files'] = len(files)
        summary['checkpoint_bytes'] = sum(f.stat().st_size for f in files)
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
        summary['second_stop'] = stop(node, 'recovery')
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
