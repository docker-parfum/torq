const nonSummableFields: Array<string> = ["alias", "pub_key", "color"];
const arrayAggKeys: Array<string> = ["channelId", "channel_point", "shortChannelId", "chan_id"];

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function useGroupBy<T>(data: Array<any>, by: string | undefined): Array<T> {
  if (by !== "peers") {
    return data;
  }

  const summedPubKey: typeof data = [];

  for (const chan of data) {
    const pub_key = String(chan["pub_key" as keyof typeof chan]);

    const summedChan = summedPubKey.find((sc) => sc["pub_key" as keyof typeof sc] == pub_key);
    if (!summedChan) {
      summedPubKey.push(chan);
      continue;
    }

    for (const key of Object.keys(chan)) {
      const value = chan[key as keyof typeof chan];

      if (nonSummableFields.includes(key)) {
        continue;
      }

      // Values fround in arrayAggKeys should be converted to an array of values
      if (arrayAggKeys.includes(key)) {
        // If the previous result is not already an Array, create a new one
        if (!Array.isArray(summedChan[key as keyof typeof summedChan])) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          (summedChan as { [key: string]: any })[key] = [summedChan[key as keyof typeof summedChan], value];
          continue;
        }

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        (summedChan as { [key: string]: any })[key] = [...summedChan[key as keyof typeof summedChan], value];
        continue;
      }

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (summedChan as { [key: string]: any })[key] = (summedChan[key as keyof typeof summedChan] as number) + value;
    }
  }

  return summedPubKey;
}
