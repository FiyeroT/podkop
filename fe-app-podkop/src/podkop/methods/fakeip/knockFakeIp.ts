import { FAKEIP_CHECK_SUFFIX } from '../../../constants';

// Opens a connection to a name made up for this check. The name does not exist: the
// request fails either way, and the backend tells whether its DNS question and the
// connection reached sing-box (check_fakeip_client with the label).
export function knockFakeIp(): { label: string; done: Promise<void> } {
  const label = Array.from(crypto.getRandomValues(new Uint8Array(10)), (byte) =>
    byte.toString(16).padStart(2, '0'),
  ).join('');
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 3000);
  const done = fetch(`https://${label}.${FAKEIP_CHECK_SUFFIX}/`, {
    mode: 'no-cors',
    cache: 'no-store',
    signal: controller.signal,
  })
    .then(
      () => undefined,
      () => undefined,
    )
    .finally(() => clearTimeout(timer));

  return { label, done };
}
