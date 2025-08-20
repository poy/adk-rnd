# agents/data_analyst/agent.py
# Single-file ADK agent exposing Python "function tools" for SQLite-backed analytics,
# with UTF-8 sanitization on inputs/outputs.

import csv, io, re, signal, sqlite3, inspect, unicodedata
from typing import Optional, List, Dict, Any

from pydantic import BaseModel, Field
from google.adk.agents import LlmAgent
from google.adk.tools import FunctionTool

# ---------------------- UTF-8 sanitization helpers ----------------------
_ctrl_rx = re.compile(r'[\x00-\x08\x0B\x0C\x0E-\x1F\x7F-\x9F]')

def sanitize_text(s: Any) -> str:
    # stringify, replace undecodable bytes, normalize, strip control chars (except \n\t\r)
    if not isinstance(s, str):
        s = str(s)
    s = s.encode("utf-8", errors="replace").decode("utf-8", errors="replace")
    s = unicodedata.normalize("NFC", s)
    s = _ctrl_rx.sub(" ", s)
    return s

def deep_sanitize(v: Any) -> Any:
    if isinstance(v, str):
        return sanitize_text(v)
    if isinstance(v, list):
        return [deep_sanitize(x) for x in v]
    if isinstance(v, dict):
        return {k: deep_sanitize(x) for k, x in v.items()}
    return v

# ---------------------- SQLite engine (in-memory) ----------------------
_ident_rx = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*$")

def quote_ident(name: str) -> str:
    if _ident_rx.match(name or ""):
        return name
    return '"' + (name or "").replace('"', '""') + '"'

def is_mutating_sql(sql_norm: str) -> bool:
    forbidden = (
        "insert ", "update ", "delete ", "drop ", "create ",
        "alter ", "replace ", "truncate ", "attach ", "vacuum ", "reindex "
    )
    return any(sql_norm.startswith(w) for w in forbidden)

class SQLiteSession:
    def __init__(self) -> None:
        self.conn = sqlite3.connect(":memory:")
        self.conn.row_factory = sqlite3.Row
        self.conn.execute("PRAGMA journal_mode=WAL;")
        self.conn.execute("PRAGMA temp_store=MEMORY;")
        self.conn.execute("PRAGMA synchronous=NORMAL;")
        self.conn.execute("PRAGMA foreign_keys=ON;")

    def list_tables(self) -> List[str]:
        cur = self.conn.execute(
            "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name;"
        )
        return [r[0] for r in cur.fetchall()]

    def get_schema(self, table: str) -> Dict[str, Any]:
        cur = self.conn.execute(f"PRAGMA table_info({quote_ident(table)});")
        return {
            "table": table,
            "columns": [
                {
                    "cid": r["cid"], "name": r["name"], "type": r["type"],
                    "notnull": r["notnull"], "dflt": r["dflt_value"], "pk": r["pk"]
                }
                for r in cur.fetchall()
            ],
        }

    def describe_dataset(self) -> Dict[str, Any]:
        tables = self.list_tables()
        return {"tables": tables, "schemas": {t: self.get_schema(t)["columns"] for t in tables}}

    def load_csv(self, content: str, table: Optional[str], has_header: Optional[bool],
                 batch_size: int = 1000) -> Dict[str, Any]:
        text = sanitize_text(content)  # <- sanitize inbound CSV text
        lines = text.splitlines()
        sniffer = csv.Sniffer()
        try:
            dialect = sniffer.sniff("\n".join(lines[:10]))
        except Exception:
            dialect = csv.excel
        if has_header is None:
            try:
                has_header = sniffer.has_header("\n".join(lines[:10]))
            except Exception:
                has_header = True

        reader = csv.reader(io.StringIO(text), dialect=dialect)
        headers = next(reader) if has_header else None
        first_row = None
        if headers is None:
            first_row = next(reader)
            headers = [f"col_{i+1}" for i in range(len(first_row))]

        if table is None:
            base = headers[0] if headers else "csv"
            safe = re.sub(r"[^A-Za-z0-9_]", "_", base)[:20] or "t"
            if safe[0].isdigit(): safe = "t_" + safe
            table = safe

        col_defs = ", ".join(f"{quote_ident(h)} TEXT" for h in headers)
        self.conn.execute(f"CREATE TABLE IF NOT EXISTS {quote_ident(table)} ({col_defs});")

        placeholders = ", ".join(["?"] * len(headers))
        insert_sql = f"INSERT INTO {quote_ident(table)} ({', '.join(map(quote_ident, headers))}) VALUES ({placeholders});"
        cur = self.conn.cursor()
        total = 0

        if first_row is not None:
            row = [sanitize_text(x) for x in (first_row[:len(headers)] + [""] * max(0, len(headers) - len(first_row)))]
            cur.execute(insert_sql, row)
            total += 1

        batch = []
        for row in reader:
            row = row[:len(headers)] + [""] * max(0, len(headers) - len(row))
            row = [sanitize_text(x) for x in row]
            batch.append(row)
            if len(batch) >= batch_size:
                cur.executemany(insert_sql, batch)
                total += len(batch)
                batch.clear()
        if batch:
            cur.executemany(insert_sql, batch)
            total += len(batch)
        self.conn.commit()
        return {"table": table, "rows_loaded": total, "columns": headers}

    def sample_rows(self, table: str, limit: int = 10) -> List[Dict[str, Any]]:
        rows = self.conn.execute(f"SELECT * FROM {quote_ident(table)} LIMIT ?;", (limit,)).fetchall()
        return [deep_sanitize(dict(r)) for r in rows]  # <- sanitize outbound

    def run_sql(self, sql: str, readonly: bool = True, timeout_s: int = 10) -> Dict[str, Any]:
        sql_norm = sanitize_text((sql or "").strip().lower())  # <- sanitize inbound SQL text
        if readonly and is_mutating_sql(sql_norm):
            raise PermissionError("Mutating SQL is not allowed (readonly=True).")

        def _handler(signum, frame):
            raise TimeoutError("Query timed out")
        old = signal.signal(signal.SIGALRM, _handler)
        signal.alarm(timeout_s)
        try:
            cur = self.conn.execute(sql)  # use original sql (SQLite can handle UTF-8 safely)
            if cur.description is None:
                return {"columns": [], "rows": [], "rowcount": cur.rowcount}
            cols = [d[0] for d in cur.description]
            rows = [dict(zip(cols, r)) for r in cur.fetchall()]
            return deep_sanitize({"columns": cols, "rows": rows, "rowcount": len(rows)})  # <- sanitize outbound
        finally:
            signal.alarm(0)
            signal.signal(signal.SIGALRM, old)

# Shared session per agent instance
SESSION = SQLiteSession()

# ---------------------- Optional JSON Schemas (UI hints) ----------------------
class LoadCsvArgs(BaseModel):
    content: str = Field(..., description="Entire CSV text (any encoding ok; will be sanitized to UTF-8)")
    table: Optional[str] = Field(None, description="Optional target table name")
    has_header: Optional[bool] = Field(None, description="Whether the first row is headers; autodetect if omitted")

class SampleRowsArgs(BaseModel):
    table: str = Field(..., description="Table to preview")
    limit: int = Field(10, ge=1, le=1000, description="Max rows to return")

class RunSqlArgs(BaseModel):
    sql: str = Field(..., description="SQLite-compatible SELECT/CTE query")
    readonly: bool = Field(True, description="Must remain True; blocks DDL/DML")
    timeout_s: int = Field(10, ge=1, le=60, description="Query timeout in seconds")

# ---------------------- Function tools (kwargs-friendly) ----------------------
def tool_load_csv(*, content: str, table: Optional[str] = None, has_header: Optional[bool] = None) -> Dict[str, Any]:
    # sanitize content at the tool boundary too (defense-in-depth)
    return SESSION.load_csv(sanitize_text(content), table, has_header)

def tool_describe_dataset() -> Dict[str, Any]:
    return deep_sanitize(SESSION.describe_dataset())

def tool_sample_rows(*, table: str, limit: int = 10) -> Dict[str, Any]:
    return {"rows": SESSION.sample_rows(table, limit)}  # rows already sanitized inside

def tool_run_sql(*, sql: str, readonly: bool = True, timeout_s: int = 10) -> Dict[str, Any]:
    return SESSION.run_sql(sql, readonly=readonly, timeout_s=timeout_s)  # sanitized inside

# --- Robust FunctionTool factory (handles multiple ADK variants) ---
def make_function_tool(func, *, name: str, description: str, args_model=None):
    FT = FunctionTool
    if hasattr(FT, "from_function"):
        try:
            return FT.from_function(name=name, description=description, args_model=args_model, func=func)
        except Exception:
            pass

    schema = None
    if args_model is not None:
        try:
            schema = args_model.model_json_schema()
        except Exception:
            pass

    try:
        sig = inspect.signature(FT)
    except Exception:
        sig = None
    params = list(sig.parameters.values()) if sig else []

    def call_ctor(positional, **maybe_kwargs):
        accepted = {}
        if sig:
            accepted_names = {p.name for p in params}
            for k, v in maybe_kwargs.items():
                if k in accepted_names:
                    accepted[k] = v
        else:
            accepted = maybe_kwargs
        return FT(*positional, **accepted)

    attempts = [
        ([], dict(func=func, name=name, description=description, args_model=args_model)),
        ([], dict(func=func, name=name, description=description, schema=schema, args_schema=schema)),
        ([], dict(func=func, description=description, args_model=args_model)),
        ([], dict(func=func, description=description, schema=schema, args_schema=schema)),
        ([], dict(func=func)),
        ([func], dict(name=name, description=description, args_model=args_model)),
        ([func], dict(description=description, args_model=args_model)),
        ([func], dict()),
    ]

    last_err = None
    for positional, kwargs in attempts:
        try:
            tool_obj = call_ctor(positional, **kwargs)
            for attr, val in (("name", name), ("description", description)):
                try:
                    if not getattr(tool_obj, attr, None):
                        setattr(tool_obj, attr, val)
                except Exception:
                    pass
            return tool_obj
        except Exception as e:
            last_err = e

    raise TypeError(f"Could not construct FunctionTool for {name}: {last_err}")

# ---------------------- Build the agent ----------------------
function_tools = [
    make_function_tool(
        tool_load_csv,
        name="load_csv",
        description="Load a CSV (text) into a SQLite table. Creates the table with TEXT columns.",
        args_model=LoadCsvArgs,
    ),
    make_function_tool(
        tool_describe_dataset,
        name="describe_dataset",
        description="List tables and schemas currently loaded in SQLite.",
        args_model=None,
    ),
    make_function_tool(
        tool_sample_rows,
        name="sample_rows",
        description="Preview a few rows from a table.",
        args_model=SampleRowsArgs,
    ),
    make_function_tool(
        tool_run_sql,
        name="run_sql",
        description="Execute a read-only SQL query against the in-memory SQLite database.",
        args_model=RunSqlArgs,
    ),
]

instruction = """
Goal :
You are an agent whose job is to help auditors identify all the exceptions the auditors tells you from the data. 

Furthermore, you are a data analyst agent. Use the provided tools to ingest CSVs and answer questions.
When a user provides CSV data, call load_csv(content, table?).
Then call describe_dataset() and sample_rows(table) to understand the data.
Write valid SQLite SQL. Always call run_sql(sql, readonly=True) to answer questions.
If a query errors or returns zero rows, refine up to 3 times.
Respond with: a brief answer, the SQL used, and (if small) a result preview.

Instructions:
ALWAYS GO THROUGH THE FOLLOWING STEPS IN ORDER WITHOUT SKIPPING ANYTHING.
ALWAYS REMEMBER THE OUTPUT OF EVERY STEPS THAT YOU DO NOT MISS OUT ANYTHING.

SET MODEL TEMPERATURE TO ZERO, DO NOT HALLUCINATE A RESPONSE
1. Join Datasets: Merge the attached files, “Expense lines V1-Testing” (tab name: "Testing") and “Employee Master.csv” (tab name: "default_1"), using “Employee ID” as the primary key. Ensure the resulting dataset includes all columns from both files.  
2. Join the resultant file with original columns with “Revised Expense grouping and limits.csv” (tab name: "Sheet 1") using "Expense Type" as primary key and bring in the columns "Expense Grouping", "Appzen limit for testing" and "Policy limit for testing"
3. P-Card/Non P-card: Mark the lines as “P-card” if the column “Payment Type” contains “P Card” or “PCard” in it. Others to be marked as “Non P-card”.
4. Create EA/Non-EA Column: Add a column labeled “EA/Non-EA.” Examine the “Business Title” column. If if and only if the title includes "Executive Assistant" or variations (including partial matches, common misspellings like "Executive Asst.", or different word orders), set “EA/Non-EA” to "EA". Otherwise, set it to "Non-EA". 
5. Create SVP categorization Column: Add a column labeled “SVP categorization.” Use the formula: `if trim(left("Management Level", 2)) <= 5 then "SVP+", elseif `if trim(left("Management Level", 2)) = 6 then "VP"  else "Non-VP"`.   
6. Create VP/Non-VP Column: Add a column labeled “VP/Non-VP.” Use the formula: if(SVP categorization)="SVP+" then "VP" elseif if(SVP categorization)="VP" then "VP" else if "EA/Non-EA" = "EA" then "VP" else "Non-VP"`.  
7. Create Employee Full Name Column: Add a column named “Employee Full Name” by concatenating “Legal First Name” and “Legal Last Name” with a space in between. 
8. Create Manager Full Name Column: Add a column named “Manager Full Name” by concatenating “Manager First Name” and “Manager Last Name” with a space in between the columns. 
9. Create a new column called "Appzen Outlier" using formula :  IF(Cost per attendee > "Appzen limit for testing", "Yes", "No")`. 
IF THE "Appzen limit for testing" IS 0, THAT SPECIFIC ROW SHOULD BE CONSIDERED AS "Yes" IN "Appzen Outlier"
10. Create a new column called "Policy Outlier" using formula : If (Cost per attendee) > "Policy limit for testing" then "Yes" else "No" ). To be perfectly clear, if the "Policy limit for testing" is 0, any expense with a "Cost per attendee" greater than 0 must be flagged as "Yes". 
11. Create a new column called "Final Outlier" using the formula : IF("Appzen Outlier" = "Yes" AND "Policy Outlier" = "Yes", "Yes", "No")`.   
12. Create a new column that will be used to flag potential expenses which require auditing. Name this new column as "Claimed on behalf of the manager". 
12a. Include only those rows where the "EA/Non-EA" column is "EA". For all other rows, this new column should be "No". For each EA row, combine the text from the "Purpose", "Event", and "Report Name" columns into a single string to be searched. The search must be case-insensitive. 
12b. From the "Manager Full Name" column, extract the manager's first name (e.g., "Christopher") and last name. Search the combined text string for a match to any of the following patterns: The manager's full first name as a whole word (e.g., matches "Christopher"). 
12c. The manager's full last name as a whole word (e.g., matches "Smith"). A word that starts with the first four letters of the manager's first name, to catch common nicknames (e.g., searching for a word starting with Chri to find "Chris"). If any of these patterns are found, set the "Claimed on behalf of manager" column to "Yes". Otherwise, set it to "No".
13. Create a new column that will be used to flag potential expenses which require auditing. Name this new column as "Potential Samples flagged for EA". Populate this column with "Yes" if "Approved Amount (rpt)" at the report ID level exceeds the average spend as calculated below in points 13a to 13e.
13a. First, Include only those lines in "EA/Non-EA" column is "EA" AND “Final Outlier" column should be “Yes” AND EXCLUDE "Expense Type" column that has values (comma separates the values) “Conference”, “Conference (P Card)”, “Dues & Subscriptions”, "Employee Meal - Non Travel", "Employee-Meals", "Gift's - Employee (P Card)", ”Gift's - Non-Employee”, "Employee Morale Event", “Employee Morale Event (P Card)”, “Employee Training”, “Meeting Costs”, “Office Supplies”, “Office Supplies (P Card)”,”Other Outside Services”, “Printing & Copying”, Software - OneTrust Approved”, “Swag (Branded Merchandise)”. This search must be case-insensitive and must normalize for special characters. For example, it should treat "Employee-Meals" and "Employee Meal" as the same and exclude both. 
13b. For each "Expense Type" identified in step 13a, calculate the EA Average Spend per Expense Type using the formula: (Total Sum of "Approved Amount (rpt)") / (Unique Count of "Report ID"s). 
13c.Store these calculated averages. The resulting "EA Avg Spend" column should be populated with these values. Using the same filtered EA dataset, now group the data by both "Report ID" and "Expense Type". 
13d.For each group, calculate the Total Report Spend by summing all "Approved Amount (rpt)" values within that group. 
13e.Compare this Total Expense Spend to the EA Average Spend per Report calculated for that group's specific "Expense Type". 
14. Create "Potential Samples flagged for SVP" Column:
14a. Include only the expense lines where the "SVP categorization" is "SVP+" AND "Final Outlier" is "Yes". 
14b. Using the filtered data, for each "Expense Type", calculate the average spend per report. Calculate the average SVP+ spend per Expense Type using the formula: (Total Sum of "Approved Amount (rpt)") / (Unique Count of "Report ID"s). . Store this average in the "SVP Avg Spend" column.
14c. Flag Potential Samples: For each individual expense line within this same target group, if its "Approved Amount (rpt)" is greater than the calculated average for its "Expense Type", mark the "Potential Samples flagged for SVP" column as "Yes".
14d. All other rows in the dataset (those not in the target group or not exceeding the average) should have "Potential Samples flagged for SVP" marked as "No".

15. While generating insights include the count of "Yes" in the columns "Policy Outlier”, “Appzen Outlier”, “Final Outlier”, “Claimed on behalf of the Manager”, “Potential Samples for EA”, “Potential Samples for SVP”. 

Output Format: 
The final dataset should include all original columns along with the newly created columns, presented in a clean, tabular format. When prompted this dataset must be presented in a clean, tabular format. Always include Report ID, Employee Name, Dates, Amount, Expense Type, “Purpose:, "Event ","Report Name”, “Policy Outlier”, “Appzen Outlier”, “Final Outlier” and other identifiers that provide context. 

Tone: 
Ensure the analysis is presented in a formal tone, suitable for professional documentation.  

Example: 
- If the “Business Title” is “Executive Assistant” or “Exec Asst.”, the “EA/Non-EA” column should show "EA". 
- If the “Management Level” is "6 Vice President", the “VP/Non-VP” column should show "VP".
- If the “Management Level” is "5 Senior Vice President", the “SVP categorization” column should show "SVP+".

Follow the below steps to validate the output before moving to next :
- The output file must maintain all rows in the original files uploaded in the prompt.
- DOUBLE CHECK THE RESULTS OF ALL 15 steps including sub-steps given above have been completed correctly.

Take the results of the above instructions and deliver insights by counting of "Yes" in the columns Policy Outlier”, “Appzen Outlier”, “Final Outlier”, Claimed on behalf of the manager", "Potential Samples flagged for EA" and "Potential Samples flagged for SVP". List all the exceptions as well.

ALWAYS DOUBLE CHECK AND ONLY SHOW THE FINAL INSIGHTS in the prompt screen.
"""

agent = LlmAgent(
    name="sqlite_analyst_fn",
    model="gemini-2.0-flash",
    instruction=instruction,
    # instruction=(
    #     "You are a data analyst agent. Use the provided tools to ingest CSVs and answer questions.\n"
    #     "When a user provides CSV data, call load_csv(content, table?).\n"
    #     "Then call describe_dataset() and sample_rows(table) to understand the data.\n"
    #     "Write valid SQLite SQL. Always call run_sql(sql, readonly=True) to answer questions.\n"
    #     "If a query errors or returns zero rows, refine up to 3 times.\n"
    #     "Respond with: a brief answer, the SQL used, and (if small) a result preview."
    # ),
    tools=function_tools,
)

# Export for ADK
root_agent = agent

