import { describe, expect, it } from 'vitest';
import {
  DEFAULT_MAX_DEVICES,
  deleteDevice,
  listDevices,
  resolveDeviceIdentity,
  sha256Hex32,
  touchDevice,
} from '../lib/devices';
import { createTestD1 } from '../testing/d1';

describe('devices helpers', () => {
  it('DEFAULT_MAX_DEVICES is 5', () => {
    expect(DEFAULT_MAX_DEVICES).toBe(5);
  });

  it('resolveDeviceIdentity prefers explicit header over UA', async () => {
    const a = await resolveDeviceIdentity({
      headerId: 'abcdef0123456789abcdef0123456789',
      userAgent: 'ClashMeta/1.0',
    });
    expect(a.fingerprint).toBe('abcdef0123456789abcdef0123456789');
    expect(a.deviceName).toBe('Clash');

    const b = await resolveDeviceIdentity({ userAgent: 'ClashMeta/1.0' });
    expect(b.fingerprint).toBe(await sha256Hex32('ua:ClashMeta/1.0'));
    expect(b.fingerprint).not.toBe(a.fingerprint);
  });

  it('touchDevice enforces max and allows revoke', async () => {
    const { db, raw } = createTestD1();
    raw.exec(
      "INSERT INTO users (id, username, password_hash, max_devices) VALUES (1, 'u', 'x', 2);" +
        "INSERT INTO users (id, username, password_hash, max_devices) VALUES (2, 'v', 'x', 0);",
    );
    const now = 1_700_000_000;
    const id1 = await resolveDeviceIdentity({ headerId: 'device--01-aaaaaaaaaaaa' });
    const id2 = await resolveDeviceIdentity({ headerId: 'device--02-bbbbbbbbbbbb' });
    const id3 = await resolveDeviceIdentity({ headerId: 'device--03-cccccccccccc' });

    expect((await touchDevice(db, 1, 2, id1, now)).ok).toBe(true);
    expect((await touchDevice(db, 1, 2, id2, now)).ok).toBe(true);
    const over = await touchDevice(db, 1, 2, id3, now);
    expect(over).toEqual({ ok: false, error: 'DEVICE_LIMIT_EXCEEDED' });

    // Same fingerprint refreshes without consuming a new slot
    const again = await touchDevice(db, 1, 2, id1, now + 10);
    expect(again).toMatchObject({ ok: true, isNew: false });

    const listed = await listDevices(db, 1);
    expect(listed).toMatchObject({ max_devices: 2, used: 2 });
    expect(listed.devices).toHaveLength(2);

    const firstId = listed.devices[0].id as number;
    expect(await deleteDevice(db, 1, firstId)).toBe('ok');
    expect(await deleteDevice(db, 1, firstId)).toBe('not_found');
    expect((await touchDevice(db, 1, 2, id3, now + 20)).ok).toBe(true);

    // max_devices = 0 → unlimited
    for (let i = 0; i < 6; i++) {
      const id = await resolveDeviceIdentity({ headerId: `unlimited-${i}-xxxxxxxxxxxx` });
      expect((await touchDevice(db, 2, 0, id, now)).ok).toBe(true);
    }
    expect((await listDevices(db, 2)).used).toBe(6);
  });
});
