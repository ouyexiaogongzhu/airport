# Node upgrade: rfplay-daemon → rfplay-gateway
#
# Do not wipe the VPS. Re-run the deploy script with the same tokens/ports;
# it stops the old unit, installs the new binary, and keeps /var/lib/rfplay.
#
# Quick path (on the VPS, from a fresh clone of this repo):
#
#   sudo systemctl disable --now rfplay-daemon
#   # optional: keep config for rollback
#   # sudo cp -a /etc/rfplay-daemon.json /etc/rfplay-gateway.json
#   sudo ./deploy/node-gateway/deploy-node-gateway.sh \
#        --manager-url https://api.rfplay.uk \
#        --node-token nd_... --tunnel-token eyJ... \
#        --hostname node-xx.rfplay.uk --local-port <port>
#
# Verify:
#   systemctl status rfplay-gateway
#   journalctl -u rfplay-gateway -f
#   ss -ltnH "sport = :<port>"   # should be 127.0.0.1 only
#
# Optional cleanup after a successful cutover:
#   sudo rm -f /usr/local/bin/rfplay-daemon \
#              /etc/rfplay-daemon.json \
#              /etc/systemd/system/rfplay-daemon.service
#   sudo systemctl daemon-reload
#
# Rollback: reinstall an older commit that still builds daemon/, restore
# /etc/rfplay-daemon.json, enable rfplay-daemon, disable rfplay-gateway.
