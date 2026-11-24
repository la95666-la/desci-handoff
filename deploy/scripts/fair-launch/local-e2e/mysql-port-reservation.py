#!/usr/bin/env python3
"""LOCAL-ONLY exact-loopback port reservation and independent evidence verification."""
import hashlib, json, os, re, signal, socket, stat, sys, time
DOMAIN = "JC_FL_MYSQL_PORT_RESERVATION_V1"
RELEASED_DOMAIN = "JC_FL_MYSQL_PORT_RESERVATION_RELEASED_V1"
HEX64 = re.compile(r"^[0-9a-f]{64}$")
ENDPOINT = re.compile(r"^127\.0\.0\.1:([0-9]{1,5})$")

def fail(message):
    print(f"fair-launch-local-e2e port reservation: FAIL: {message}", file=sys.stderr); raise SystemExit(1)
def process_start(pid):
    try: return open(f"/proc/{pid}/stat", encoding="ascii").read().split()[21]
    except (OSError, IndexError): return ""
def atomic(path, obj):
    tmp=f"{path}.tmp.{os.getpid()}"; fd=os.open(tmp,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    try:
        os.write(fd,(json.dumps(obj,sort_keys=True,separators=(",",":"))+"\n").encode()); os.fsync(fd)
    finally: os.close(fd)
    os.replace(tmp,path); os.chmod(path,0o600)
def digest(pid,start,endpoint,token): return hashlib.sha256(f"{DOMAIN}|{pid}|{start}|{endpoint}|{token}".encode()).hexdigest()
def load_verified(path, live=True):
    st=os.stat(path)
    if stat.S_IMODE(st.st_mode)!=0o600: fail("state file must be mode 0600")
    with open(path,encoding="utf-8") as f: obj=json.load(f)
    if set(obj)!={"pid","startTime","endpoint","token","digest"}: fail("state keys mismatch")
    pid=obj["pid"]; start=obj["startTime"]; endpoint=obj["endpoint"]; token=obj["token"]; supplied=obj["digest"]
    if not isinstance(pid,int) or pid<=1 or not isinstance(start,str) or not start.isdigit(): fail("pid/start shape")
    m=ENDPOINT.fullmatch(endpoint) if isinstance(endpoint,str) else None
    if not m or not 1024<=int(m.group(1))<=65535: fail("endpoint shape")
    if not isinstance(token,str) or not HEX64.fullmatch(token): fail("token shape")
    expected=digest(pid,start,endpoint,token)
    if not isinstance(supplied,str) or not HEX64.fullmatch(supplied) or supplied!=expected: fail("digest mismatch")
    if live and process_start(pid)!=start: fail("worker pid/start mismatch")
    return obj
def reserve(path):
    old=os.umask(0o077)
    try:
        sock=socket.socket(socket.AF_INET,socket.SOCK_STREAM); sock.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,0); sock.bind(("127.0.0.1",0)); sock.listen(1)
        pid=os.getpid(); start=process_start(pid)
        if not start: fail("cannot fingerprint reservation process")
        endpoint=f"127.0.0.1:{sock.getsockname()[1]}"; token=os.urandom(32).hex(); value=digest(pid,start,endpoint,token)
        atomic(path,{"pid":pid,"startTime":start,"endpoint":endpoint,"token":token,"digest":value}); print(f"READY {endpoint}",flush=True)
        if os.environ.get("JC_FL_E2E_TEST_RESERVATION_RESISTANT_WORKER")=="1" or os.environ.get("JC_FL_E2E_TEST_RESERVATION_DELAY_EXIT")=="1":
            signal.signal(signal.SIGTERM,signal.SIG_IGN)
            while True: time.sleep(1)
        line=sys.stdin.readline().rstrip("\n")
        if line!=f"RELEASE {token}": sock.close(); fail("control channel closed or invalid before release")
        sock.close(); print("RELEASED",flush=True)
    finally: os.umask(old)
def verify(path):
    o=load_verified(path,True)
    for k in ("pid","startTime","endpoint","digest","token"): print(f"{k.upper()}={o[k]}")
def released(path,out):
    o=load_verified(path,False)
    evidence={"domain":RELEASED_DOMAIN,"workerPid":o["pid"],"workerStartTime":o["startTime"],"endpoint":o["endpoint"],"reservationDigest":o["digest"]}
    raw=f"{RELEASED_DOMAIN}|{o['pid']}|{o['startTime']}|{o['endpoint']}|{o['digest']}"
    evidence["evidenceDigest"]=hashlib.sha256(raw.encode()).hexdigest(); atomic(out,evidence)
def verify_released(path):
    st=os.stat(path)
    if stat.S_IMODE(st.st_mode)!=0o600: fail("released evidence mode")
    with open(path,encoding="utf-8") as f:o=json.load(f)
    keys={"domain","workerPid","workerStartTime","endpoint","reservationDigest","evidenceDigest"}
    if set(o)!=keys or o["domain"]!=RELEASED_DOMAIN: fail("released evidence keys/domain")
    if not isinstance(o["workerPid"],int) or o["workerPid"]<=1 or not str(o["workerStartTime"]).isdigit(): fail("released pid/start")
    if not ENDPOINT.fullmatch(str(o["endpoint"])) or not HEX64.fullmatch(str(o["reservationDigest"])): fail("released endpoint/digest")
    raw=f"{RELEASED_DOMAIN}|{o['workerPid']}|{o['workerStartTime']}|{o['endpoint']}|{o['reservationDigest']}"
    expected=hashlib.sha256(raw.encode()).hexdigest()
    if o["evidenceDigest"]!=expected: fail("released evidence digest")
    for k in ("workerPid","workerStartTime","endpoint","reservationDigest","evidenceDigest"): print(f"{k.upper()}={o[k]}")
def main():
    if len(sys.argv)<3: fail("mode and file required")
    mode=sys.argv[1]
    if mode=="reserve" and len(sys.argv)==3: reserve(os.path.abspath(sys.argv[2]))
    elif mode=="verify" and len(sys.argv)==3: verify(os.path.abspath(sys.argv[2]))
    elif mode=="release-evidence" and len(sys.argv)==4: released(os.path.abspath(sys.argv[2]),os.path.abspath(sys.argv[3]))
    elif mode=="verify-release" and len(sys.argv)==3: verify_released(os.path.abspath(sys.argv[2]))
    else: fail("invalid usage")
if __name__=="__main__":
    signal.signal(signal.SIGTERM,lambda *_: (_ for _ in ()).throw(SystemExit(143))); main()
