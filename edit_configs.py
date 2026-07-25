import re

cfg = r"$HOME/mcchain\testnet\config\config.toml"
app = r"$HOME/mcchain\testnet\config\app.toml"

c = open(cfg, encoding="utf-8").read()
c = re.sub(r'timeout_commit = "[^"]*"', 'timeout_commit = "4s"', c)
open(cfg, "w", encoding="utf-8").write(c)

a = open(app, encoding="utf-8").read()
a = re.sub(r'minimum-gas-prices = "[^"]*"', 'minimum-gas-prices = "0umc"', a)
open(app, "w", encoding="utf-8").write(a)

print("config.toml timeout_commit =", re.search(r'timeout_commit = "[^"]*"', c).group(0))
print("app.toml minimum-gas-prices =", re.search(r'minimum-gas-prices = "[^"]*"', a).group(0))
