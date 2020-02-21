SecretShop: a small Gemini server.

# Features
* Multi-site hosting
* Also supports simple Gopher hosting
* Fully compliant with Jetforce diagnostics
* Probably won't kill your computer

# Configuration
SecretShop looks in it's current running directory and /etc/secretshop for it's config file.
Configuration is in config/yaml in one of the above directories. See the sample config for more details
but a standard file looks like such:

---
active_capsules:
        - localhost
localhost:
        Hostname: "localhost"
        Port: "1965"
        RootDir: "/var/gemini"
        CGIDir: "/var/gemini/cgi"
        KeyFile: "localhost.key"
        CertFile: "localhost.crt"

Please note that CGIDir currently not used (waiting on spec clarification).

# Installation
Running "make install" will install to /usr/local/bin by default.

Running "make service" will install to /usr/local/bin and also install the systemd service file

# Uninstall
Simply run "make uninstall"

# Running
Either run the executable directly or use the SystemD unit file