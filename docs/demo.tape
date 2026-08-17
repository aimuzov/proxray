# The recording in the README.
#
#   brew install vhs ttyd
#   go build -o proxray .
#   cd docs && vhs demo.tape
#
# Unlike a real session, this one is self-contained: docs/demo/panel.go serves an
# invented subscription on 127.0.0.1, and the shell runs with HOME pointed at
# docs/demo-home, so the recorder's own subscriptions, tokens and servers never
# reach the screen. Every command is the real one — `sub add` fetches over HTTP and
# `connect` starts the embedded xray-core and listens on the two ports — but the
# servers behind those links do not exist, so no traffic would actually flow.
#
# The hidden warm-up downloads the geoip/geosite databases the RU bypass needs. They
# are cached in docs/demo-home for a day, so only the first recording pays for it —
# and pays visibly, hence the generous sleep.

Output demo.gif

Set Shell "bash"
Set FontFamily "JetBrainsMonoNL Nerd Font"
Set FontSize 15
Set Width 1200
Set Height 560
Set Padding 18
Set Theme "Catppuccin Mocha"
Set TypingSpeed 75ms
Set Framerate 24

Hide

# The fake panel starts in the recorder's own shell, where go is on PATH; the demo
# profile below deliberately drops everything that is not needed on screen.
Type "go run demo/panel.go &"
Enter
Sleep 8s

# A store left over from an earlier recording would make `sub add` an update; the
# geo databases next to it are kept, they are the expensive part.
Type "find demo-home -name state.json -delete"
Enter
Sleep 1s

Type "env HOME=$PWD/demo-home XDG_CONFIG_HOME=$PWD/demo-profile fish -i"
Enter
Sleep 3s

# Warm-up: fetch once and connect once, so the geo databases are on disk before the
# camera is on, then leave no trace of it in the store.
Type "proxray sub add http://127.0.0.1:8099/sub/demo --name warmup"
Enter
Sleep 3s
Type "proxray connect --sub warmup"
Enter
Sleep 60s
Ctrl+C
Sleep 2s
Type "proxray sub rm warmup"
Enter
Sleep 1s
Type "clear"
Enter
Sleep 1s

Show

Sleep 1s

# a subscription URL in, a list of servers out
Type "proxray sub add http://127.0.0.1:8099/sub/demo --name myvpn"
Enter
Sleep 3s

# traffic and expiry come from the headers the panel sent
Type "proxray sub list"
Enter
Sleep 4s

# the servers themselves, including the one xray-core cannot dial
Type "proxray list"
Enter
Sleep 5s

# Russian traffic goes direct by default, so ozon.ru and friends keep working
Type "proxray route"
Enter
Sleep 3s

Type "clear"
Enter
Sleep 500ms

# the same thing without remembering any of it: pick a server, pick a method
Type "proxray -i"
Enter
Sleep 2s
Type "ger"
Sleep 2s
Enter
Sleep 2s

# the second method is the one that needs no sudo
Down
Sleep 2s
Enter
Sleep 5s

Ctrl+C
Sleep 3s
