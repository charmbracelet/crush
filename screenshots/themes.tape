# Theme Screenshots for Crush
#
# This VHS tape captures screenshots of each built-in theme by launching
# Crush, opening the command palette, navigating to Switch Theme, and
# cycling through each theme with live preview.
#
# Prerequisites:
#   - The `crush` binary must be in the working directory or PATH
#   - At least one provider API key must be configured
#   - VHS must be installed (https://github.com/charmbracelet/vhs)
#
# Usage:
#   cd crush
#   vhs screenshots/themes.tape

Output screenshots/themes.gif

Require crush

Set Shell bash
Set FontSize 14
Set FontFamily "JetBrains Mono"
Set Width 1200
Set Height 700
Set Padding 20
Set WindowBar Colorful
Set BorderRadius 8
Set TypingSpeed 50ms
Set CursorBlink false

# Launch Crush
Type "./crush"
Enter
Sleep 5s
Screenshot screenshots/charm-default.png

# Open command palette
Ctrl+K
Sleep 1s

# Search for Switch Theme
Type "theme"
Sleep 500ms
Enter
Sleep 1s

# Now in the theme picker — first item should be selected.
# The list is sorted alphabetically, so the order is:
#   catppuccin-latte, catppuccin-macchiato, catppuccin-mocha,
#   charm, dracula, everforest-dark, everforest-light,
#   gruvbox-dark, gruvbox-light, kanagawa-lotus, kanagawa-wave,
#   material-darker, material-lighter, nord, one-dark,
#   rose-pine, rose-pine-dawn, rose-pine-moon,
#   solarized-dark, solarized-light, tokyo-night, tokyo-night-day
#
# Live preview triggers on each arrow key movement.

# 1. catppuccin-latte (first item, already selected)
Sleep 500ms
Screenshot screenshots/catppuccin-latte.png

# 2. catppuccin-macchiato
Down
Sleep 500ms
Screenshot screenshots/catppuccin-macchiato.png

# 3. catppuccin-mocha
Down
Sleep 500ms
Screenshot screenshots/catppuccin-mocha.png

# 4. charm
Down
Sleep 500ms
Screenshot screenshots/charm.png

# 5. dracula
Down
Sleep 500ms
Screenshot screenshots/dracula.png

# 6. everforest-dark
Down
Sleep 500ms
Screenshot screenshots/everforest-dark.png

# 7. everforest-light
Down
Sleep 500ms
Screenshot screenshots/everforest-light.png

# 8. gruvbox-dark
Down
Sleep 500ms
Screenshot screenshots/gruvbox-dark.png

# 9. gruvbox-light
Down
Sleep 500ms
Screenshot screenshots/gruvbox-light.png

# 10. kanagawa-lotus
Down
Sleep 500ms
Screenshot screenshots/kanagawa-lotus.png

# 11. kanagawa-wave
Down
Sleep 500ms
Screenshot screenshots/kanagawa-wave.png

# 12. material-darker
Down
Sleep 500ms
Screenshot screenshots/material-darker.png

# 13. material-lighter
Down
Sleep 500ms
Screenshot screenshots/material-lighter.png

# 14. nord
Down
Sleep 500ms
Screenshot screenshots/nord.png

# 15. one-dark
Down
Sleep 500ms
Screenshot screenshots/one-dark.png

# 16. rose-pine
Down
Sleep 500ms
Screenshot screenshots/rose-pine.png

# 17. rose-pine-dawn
Down
Sleep 500ms
Screenshot screenshots/rose-pine-dawn.png

# 18. rose-pine-moon
Down
Sleep 500ms
Screenshot screenshots/rose-pine-moon.png

# 19. solarized-dark
Down
Sleep 500ms
Screenshot screenshots/solarized-dark.png

# 20. solarized-light
Down
Sleep 500ms
Screenshot screenshots/solarized-light.png

# 21. tokyo-night
Down
Sleep 500ms
Screenshot screenshots/tokyo-night.png

# 22. tokyo-night-day
Down
Sleep 500ms
Screenshot screenshots/tokyo-night-day.png

# Cancel out without applying
Escape
Sleep 500ms

# Quit
Ctrl+C
Sleep 1s
