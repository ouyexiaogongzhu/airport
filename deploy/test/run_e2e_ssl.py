#!/usr/bin/env python3
"""E2E test runner through HTTPS with SSL verification disabled."""
import ssl, urllib.request, sys, os

# SECURITY WARNING: This script disables SSL certificate verification to
# allow testing against self-signed or staging certificates. This must
# NEVER be used in production. In CI, use a properly signed certificate
# or add the test CA to the trust store instead.
_ctx = ssl.create_default_context()
_ctx.check_hostname = False
_ctx.verify_mode = ssl.CERT_NONE

# Monkey-patch urlopen to use unverified context
_orig_urlopen = urllib.request.urlopen
def _patched_urlopen(url, *args, **kwargs):
    kwargs.setdefault('context', _ctx)
    return _orig_urlopen(url, *args, **kwargs)
urllib.request.urlopen = _patched_urlopen

os.environ['API_BASE'] = 'https://localhost'

# Run the actual E2E test
test_path = os.path.join(os.path.dirname(__file__), 'e2e_test.py')
if not os.path.exists(test_path):
    test_path = os.path.join(os.path.dirname(os.path.abspath(__file__)), 'deploy/test/e2e_test.py')

exec(open(test_path).read())
