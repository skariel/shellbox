run "tree" to get to know the project. Just tree, no other params

then run:
grep -rn -E "^func\s*(\([^)]+\))?\s*[a-zA-Z_][a-zA-Z0-9_]*\s*\(" . --include="*.go" | grep -v

use the LSP to edit files.

thats' it :)

