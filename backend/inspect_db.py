import sqlite3

db_path = 'data/users/cee19697-23d0-44f1-8e98-1460239ed921/projects.db'
try:
    con = sqlite3.connect(db_path)
    tables = [r[0] for r in con.execute("SELECT name FROM sqlite_master WHERE type='table'").fetchall()]
    print('Tables:', tables)
    
    # List recent projects
    if 'projects' in tables:
        projects = con.execute("SELECT id, name FROM projects ORDER BY id DESC LIMIT 5").fetchall()
        print('Projects:', projects)
    
    # List recent files
    for tname in ['code_files', 'files', 'project_files']:
        if tname in tables:
            rows = con.execute(f"SELECT project_id, path, substr(content,1,120) FROM {tname} ORDER BY rowid DESC LIMIT 5").fetchall()
            for r in rows:
                print(f'  {tname}: proj={r[0]} path={r[1]} | {r[2]}')
    con.close()
except Exception as e:
    print('ERROR:', e)
