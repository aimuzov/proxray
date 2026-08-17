# The fish profile the README recording runs in: none of the recorder's own prompt,
# bindings or colours — just proxray, built from this checkout.

set -g fish_greeting

set -l here (dirname (status filename))
set -l repo (realpath $here/../../..)

# PATH is rebuilt from scratch rather than extended. proxray comes from the checkout
# (`go build -o proxray .`), and everything else is the system's. The recorder's own
# tools stay out — fzf in particular: proxray uses the fzf binary when it finds one
# (see internal/cli/fuzzy.go) and its embedded finder otherwise, and the recording
# should show the same finder on every machine, not whatever the recorder installed.
set -gx PATH $repo /usr/bin /bin /usr/sbin /sbin

# No autosuggestions while recording: they replay the shell history, and a proxray
# command line carries subscription URLs that have no business in a gif.
set -g fish_autosuggestion_enabled 0

# Catppuccin Mocha, to match the theme vhs records with
set -g fish_color_normal cdd6f4
set -g fish_color_command 89b4fa
set -g fish_color_param f2cdcd
set -g fish_color_comment 7f849c
set -g fish_color_autosuggestion 6c7086
set -g fish_pager_color_prefix f5e0dc --bold
set -g fish_pager_color_completion cdd6f4
set -g fish_pager_color_description 9399b2
set -g fish_pager_color_selected_background --background=313244

function fish_prompt
    set_color brblack
    echo -n '~ '
    set_color normal
end
