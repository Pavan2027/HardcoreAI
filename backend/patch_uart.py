"""
Patches src/main.c in the most recent project via the backend API.
Adds __HAL_RCC_USART2_CLK_ENABLE() before HAL_UART_Init if missing.
"""
import urllib.request, urllib.error, json, sys

BASE = "http://127.0.0.1:62018"

# 1. Get auth token (dev mode uses fixed token)
def get_token():
    req = urllib.request.Request(f"{BASE}/api/auth/dev-token", method="POST")
    try:
        with urllib.request.urlopen(req) as r:
            return json.loads(r.read())["token"]
    except:
        return "dev"  # fallback

def api(path, method="GET", body=None, token="dev"):
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(f"{BASE}{path}", data=data, headers=headers, method=method)
    with urllib.request.urlopen(req) as r:
        return json.loads(r.read())

token = get_token()

# 2. Get projects
projects = api("/api/projects", token=token)
if not projects:
    print("No projects found")
    sys.exit(1)

latest = projects[-1]  # most recently created
project_id = latest["id"]
print(f"Project: {latest.get('name', '?')} (id={project_id})")

# 3. Get files
files = api(f"/api/projects/{project_id}/files", token=token)
main_c = next((f for f in files if f["path"] == "src/main.c"), None)
if not main_c:
    print("No src/main.c found")
    sys.exit(1)

content = main_c["content"]
print(f"Current src/main.c length: {len(content)} bytes")

# 4. Patch: add USART2 clock enable if missing
if "__HAL_RCC_USART2_CLK_ENABLE" in content:
    print("__HAL_RCC_USART2_CLK_ENABLE already present — no patch needed")
else:
    # Insert before "huart2.Instance = USART2"
    old = "  huart2.Instance = USART2;"
    new = "  __HAL_RCC_USART2_CLK_ENABLE();\n\n  huart2.Instance = USART2;"
    if old not in content:
        print("Could not find insertion point 'huart2.Instance = USART2;'")
        sys.exit(1)
    patched = content.replace(old, new, 1)
    result = api(
        f"/api/projects/{project_id}/files/src/main.c",
        method="PUT",
        body={"content": patched},
        token=token
    )
    print("Patched src/main.c — USART2 clock enable added")
    print("Now click Build → Flash in the UI")
