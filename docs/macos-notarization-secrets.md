# macOS Signing and Notarization Secrets

The release workflow needs six GitHub repository secrets. They allow GitHub Actions to sign the Control Panel with your personal Developer ID certificate and ask Apple to notarize the DMG.

None of these values is committed to the repository. The setup script uploads them with `gh secret set`.

| GitHub secret | What it is | Where it comes from |
| --- | --- | --- |
| `MACOS_CERTIFICATE_BASE64` | A text-safe copy of your Developer ID Application `.p12` certificate, including its private key. | The setup script encodes `~/Documents/cert/developerID_application.p12`. Do not use the separate `.base64` file. |
| `MACOS_CERTIFICATE_PASSWORD` | The password you chose when exporting the `.p12` file. It lets the workflow import the certificate into a temporary keychain. | The password used during the `.p12` export. |
| `MACOS_KEYCHAIN_PASSWORD` | A temporary password for the keychain created inside the GitHub runner. | Generated randomly by the setup script. You do not need to remember it. |
| `APPLE_ID` | The email address of the Apple ID enrolled in the Apple Developer Program. | Your usual Apple Developer login email address. |
| `APPLE_TEAM_ID` | Your personal Apple Developer team identifier. It is not an organization account. | Open [Apple Developer Account](https://developer.apple.com/account/) and look under **Membership details**. |
| `APPLE_APP_SPECIFIC_PASSWORD` | A one-purpose password that lets the workflow submit the signed DMG to Apple's notarization service. It is not your ordinary Apple ID password. | Create one at [Apple Account](https://account.apple.com/) under **Sign-In and Security** > **App-Specific Passwords**. Give it a label such as `Control Panel GitHub Actions`. |

## Non-secret signing identity

`APPLE_SIGNING_IDENTITY` is not a password or private key. It is the public name printed on your certificate, for example:

```text
Developer ID Application: Jane Doe (ABCDEFGHIJ)
```

The setup script stores it as a GitHub repository variable, rather than a secret.

## What Apple receives

The workflow signs the application with the certificate, creates the DMG, and submits the completed DMG to Apple's automated notarization service using the Apple ID and app-specific password. Apple normally completes the automated check in minutes. The workflow staples Apple's approval to the DMG before publishing it.

Users can then download the release and open the application normally, without changing System Settings or using Control-click > Open.

## Uploading the values

After creating the app-specific password, run this from the controlpanel repository:

```bash
bash scripts/setup-macos-notarization-secrets.sh \
  "$HOME/Documents/cert/developerID_application.p12" \
  "Developer ID Application: Your Name (YOUR_PERSONAL_TEAM_ID)" \
  "owlcms/owlcms-controlpanel"
```

The script asks for the other private values without echoing passwords to the terminal. It never prints or stores them in this repository.