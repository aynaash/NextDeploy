# ⚡ Zed + Vim Mode Master Cheatsheet

Editor: Zed  
Mode: Vim Mode Enabled  
Goal: Maximum Speed, Minimum Configuration

---

# 🧭 BASIC MOVEMENT

## Character Movement
```
h j k l        → left / down / up / right
```

## Word Movement
```
w              → next word start
b              → previous word start
e              → end of word
```

## Line Movement
```
0              → beginning of line
^              → first non-blank character
$              → end of line
```

## File Movement
```
gg             → top of file
G              → bottom of file
<number>G      → go to line number
%              → jump to matching bracket
```

---

# 🔁 JUMP HISTORY

```
ctrl-o         → go back
ctrl-i         → go forward
ctrl-tab       → switch to previously opened file
```

Use `ctrl-o` after `gd` or search jumps.

---

# 🧠 LSP NAVIGATION (CODE INTELLIGENCE)

```
gd             → go to definition
gD             → go to declaration
gy             → go to type definition
gI             → go to implementation
gA             → find all references
gs             → find symbol in file
gS             → find symbol in project
g.             → code actions
gh             → hover info
]d             → next diagnostic
[d             → previous diagnostic
```

---

# 🌲 STRUCTURAL NAVIGATION (TREE-SITTER)

## Move Between Structures
```
]m             → next function
[m             → previous function
]]             → next class/section
[[             → previous class/section
```

## Text Objects
```
cif            → change inside function
caf            → change around function
ciC            → change inside class
af             → around function
if             → inside function
ac             → around class
ic             → inside class
```

---

# 🔍 SEARCH

```
/text          → search forward
?text          → search backward
n              → next match
N              → previous match
```

---

# 🔁 REPLACE

Global by default in Zed.

```
:%s/foo/bar/
```

With capture groups:
```
:%s/(foo)(bar)/$1/
```

Note:
- Use `$1` instead of `\1`
- Global replace is default

---

# 🗂 FILE MANAGEMENT

## Save / Quit
```
:w             → save
:q             → quit
:wq            → save & quit
:qa            → quit all
:bd            → close current file
```

## Split Panes
```
:vs            → vertical split
:sp            → horizontal split
:tabnew        → new tab
:tabn          → next tab
:tabp          → previous tab
```

---

# 📁 FILE OPERATIONS (PROJECT PANEL)

## Open Project Panel
```
ctrl-shift-e
```

## Inside Project Panel
```
n              → new file
F2             → rename file
Delete         → delete file
```

---

# 🔎 PROJECT & PANELS

```
g/             → project-wide search
:E             → open project panel
:G             → open git panel
:te            → open terminal
:AI            → open AI panel
```

---

# 🎯 MULTI-CURSOR (ZED POWER FEATURE)

```
gl             → add cursor to next match
gL             → add cursor to previous match
ga             → select all matches
gA             → cursor at end of each selected line
gI             → cursor at start of each selected line
```

---

# ✏️ EDITING

```
dd             → delete line
yy             → yank line
p              → paste
u              → undo
ctrl-r         → redo
J              → join lines
```

## Surround
```
ys             → add surround
cs             → change surround
ds             → delete surround
```

---

# 🧩 GIT NAVIGATION

```
]c             → next git change
[c             → previous git change
do             → expand diff hunk
dp             → restore change
```

---

# ⚡ MINIMAL DAILY CORE

If you only remember these, you're fast:

```
h j k l
w b e
gg G %
gd
gA
g.
ctrl-o
:vs
:w
g/
```

---

# 🧠 MENTAL MODEL

Neovim = configurable system  
Zed = fast execution environment  

Focus on:
- Architecture
- Systems
- Shipping
- Not editor tweaking

---

End of file.
