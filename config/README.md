# 🍪 YouTube Cookies Guide

This system allows `yt-dlp` to authenticate with YouTube, bypassing age restrictions, region locks, and bot detection.

## 🚀 Setup Methods

### Method 1: Remote via Batbin (Recommended)

This method keeps your sensitive cookies out of your Git history and allows for easy updates.

1. **Export Cookies:** Use a browser extension like [Get cookies.txt](https://chrome.google.com/webstore/detail/get-cookiestxt/) to export your YouTube cookies in Netscape format.
2. **Upload to Batbin:** Paste the cookie text into [batbin.me](https://batbin.me) and create a paste.
3. **Configure Bot:** Copy the paste link and set it in your environment:
   ```bash
   COOKIES_LINK="https://batbin.me/paste_id"
   ```
   *Note: You can provide multiple space-separated URLs for load balancing.*

### Method 2: Render Secret File (Private / Recommended for Render)

1. In your Render service, open **Environment** → **Secret Files** → **Add Secret File**.
2. Set the filename to `cookies.txt` and paste the Netscape-format cookie contents.
3. Add this environment variable to the same service:
   ```bash
   COOKIES_FILE=/etc/secrets/cookies.txt
   ```
4. Save changes and deploy. The Docker image is configured to let `appuser` read Render's secret-file group (GID 1000).

The application reads this file directly; it does not copy the cookie contents into the repository or image.

### Method 3: Manual Local Files

Place any Netscape-formatted cookie file directly into the `config/cookies/` directory with a `.txt` extension. The bot automatically detects and uses all `.txt` files in this folder.

## 🛠️ Troubleshooting

### "Sign in to confirm you're not a bot"
This usually means your cookies have expired or were exported incorrectly.
- **Solution:** Re-export fresh cookies from your browser and update your Batbin paste or local files.

### "No cookie files found"
The bot couldn't find any valid `.txt` files in `config/cookies/`.
- **Solution:** Verify that your `COOKIES_LINK` is correct or that your local files have the `.txt` extension and are not named `example.txt`.

## 💡 Best Practices

- **Security:** Never commit your real cookie files to Git or publish them in a public paste. On Render, use a Secret File with `COOKIES_FILE`.
- **Maintenance:** YouTube cookies expire periodically. Refresh them every 2-4 days or if you notice download failures.
- **Redundancy:** Use multiple cookie files from different accounts to avoid rate limits.

---
**Note:** For more help, join our [Support Chat](https://t.me/MeowClone).
