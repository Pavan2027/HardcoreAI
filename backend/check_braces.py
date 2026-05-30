import re

with open('solver.py', encoding='utf-8') as f:
    src = f.read()

# Find _AGENT_SYSTEM template
start = src.index('_AGENT_SYSTEM = """') + len('_AGENT_SYSTEM = """')
end = src.index('"""', start)
template = src[start:end]

print("=== Trying .format() ===")
try:
    result = template.format(tools="TEST", current_code="CODE", has_docs="YES", problem="PROB")
    print("OK - no format errors")
except KeyError as e:
    print(f"KeyError: {e}")
    # Find the offending brace
    key = str(e).strip("'")
    idx = template.find('{' + key)
    print(f"Found at index {idx}: {repr(template[max(0,idx-30):idx+len(key)+50])}")

# Also check _AGENT_USER
start2 = src.index('_AGENT_USER = """') + len('_AGENT_USER = """')
end2 = src.index('"""', start2)
template2 = src[start2:end2]
print("\n=== Checking _AGENT_USER ===")
try:
    template2.format(tools="T", current_code="C", has_docs="H", problem="P")
    print("OK")
except KeyError as e:
    print(f"KeyError: {e}")
