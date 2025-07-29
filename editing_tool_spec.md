# Tool Specification: `edit`

## 1. Tool Definition

**Name:** `edit`

**Description:** Provides a comprehensive set of commands for programmatically editing text files. It offers fine-grained control over file modifications, including positional inserts, deletes, and pattern-based replacements.

**Input Schema:**

The tool takes a single JSON object as input with the following properties:

```json
{
  "command": "string",
  "arguments": "object"
}
```

*   `command` (string, required): The editing operation to perform. Must be one of: `insert`, `delete`, `replace`, `replaceAll`, `search`.
*   `arguments` (object, required): A JSON object containing the arguments specific to the chosen command.

---

## 2. Commands and Arguments

This section details the `arguments` object required for each command and the expected output.

### 2.1. Command: `insert`

**Description:** Inserts content at a specific position in a file.

**Arguments Schema:**

```json
{
  "file_path": "string",
  "content": "string",
  "position": "Position"
}
```

*   `file_path` (string, required): The absolute path to the file.
*   `content` (string, required): The text to insert.
*   `position` (Position, required): The location for the insertion.

**Output Schema:**

```json
{
  "status": "string",
  "message": "string"
}
```
*   `status`: "success"
*   `message`: "Content inserted successfully."

---

### 2.2. Command: `delete`

**Description:** Deletes a range of text from a file.

**Arguments Schema:**

```json
{
  "file_path": "string",
  "range": "Range"
}
```

*   `file_path` (string, required): The absolute path to the file.
*   `range` (Range, required): The range of text to delete.

**Output Schema:**

```json
{
  "status": "string",
  "message": "string"
}
```
*   `status`: "success"
*   `message`: "Text deleted successfully."

---

### 2.3. Command: `replace`

**Description:** Replaces a range of text with new content.

**Arguments Schema:**

```json
{
  "file_path": "string",
  "range": "Range",
  "content": "string"
}
```

*   `file_path` (string, required): The absolute path to the file.
*   `range` (Range, required): The range of text to replace.
*   `content` (string, required): The new content to insert.

**Output Schema:**

```json
{
  "status": "string",
  "message": "string"
}
```
*   `status`: "success"
*   `message`: "Text replaced successfully."

---

### 2.4. Command: `replaceAll`

**Description:** Replaces all occurrences of a pattern with new content.

**Arguments Schema:**

```json
{
  "file_path": "string",
  "pattern": "string",
  "replacement": "string"
}
```

*   `file_path` (string, required): The absolute path to the file.
*   `pattern` (string, required): The regular expression to search for.
*   `replacement` (string, required): The string to replace each match with. Capture groups can be referenced (e.g., `$1`).

**Output Schema:**

```json
{
  "status": "string",
  "replacements_made": "integer"
}
```
*   `status`: "success"
*   `replacements_made`: The number of replacements performed.

---

### 2.5. Command: `search`

**Description:** Searches for a pattern in a file and returns all matches.

**Arguments Schema:**

```json
{
  "file_path": "string",
  "pattern": "string"
}
```

*   `file_path` (string, required): The absolute path to the file.
*   `pattern` (string, required): The regular expression to search for.

**Output Schema:**

```json
{
  "matches": "array"
}
```
*   `matches` (array of Match): A list of all non-overlapping matches found.

---

## 3. Data Structures

These are the reusable data structures for the command arguments and return values.

### 3.1. `Position`

**Description:** Represents a single point in a file. Either `offset` or both `line` and `column` must be provided.

**JSON Schema:**
```json
{
  "line": "integer",
  "column": "integer",
  "offset": "integer"
}
```
*   `line` (integer, optional): The 1-based line number.
*   `column` (integer, optional): The 1-based column number.
*   `offset` (integer, optional): The 0-based character offset from the start of the file.

### 3.2. `Range`

**Description:** Represents a range of text in a file.

**JSON Schema:**
```json
{
  "start": "Position",
  "end": "Position"
}
```
*   `start` (Position, required): The start position of the range (inclusive).
*   `end` (Position, required): The end position of the range (exclusive).

### 3.3. `Match`

**Description:** Represents a single match from a search operation.

**JSON Schema:**
```json
{
  "range": "Range",
  "groups": ["string"]
}
```
*   `range` (Range, required): The range of the matched text.
*   `groups` (array of strings, required): The captured groups from the regular expression. The first element is the full match.