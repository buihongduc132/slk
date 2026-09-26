import re

with open('internal/ui/app.go', 'r') as f:
    content = f.read()

# Make sure reduceZoom is in the list
if 'reduceZoom,' not in content:
    content = content.replace(
        '\treduceMouse,\n',
        '\treduceMouse,\n\t\treduceZoom,\n'
    )

with open('internal/ui/app.go', 'w') as f:
    f.write(content)
