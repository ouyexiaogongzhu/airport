-- XHTTP-only: coerce all nodes off ws/tcp/legacy network values.
-- Column default in 0001 remains historical; inserts always set network explicitly.
UPDATE nodes SET network = 'xhttp' WHERE network IS NULL OR network != 'xhttp';
