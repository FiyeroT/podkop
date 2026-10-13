import { FAKEIP_CHECK_SUFFIX } from '../../../constants';

// Opens a connection to a name made up for a check and returns the label of the name. The
// name does not exist: the request fails either way, sooner or later, and nothing waits
// for that. Whether its DNS question and the connection reached sing-box is asked of the
// backend, with the label (check_fakeip_client, check_nft_rules).
export function knockFakeIp(): string {
  const label = Array.from(crypto.getRandomValues(new Uint8Array(10)), (byte) =>
    byte.toString(16).padStart(2, '0'),
  ).join('');

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 3000);

  fetch(`https://${label}.${FAKEIP_CHECK_SUFFIX}/`, {
    mode: 'no-cors',
    cache: 'no-store',
    signal: controller.signal,
  })
    .catch(() => undefined)
    .finally(() => clearTimeout(timer));

  return label;
}
