# Mohur privacy

Last changed 24 November 2026.

Mohur lets people at organizations that each run their own server write to each other, end to end encrypted. It is two programs: the Mohur app for Windows, and the Mohur server, `mohurd`, which each organization runs for itself. denyfirst writes both and publishes them as free software under the AGPL-3.0. denyfirst runs no Mohur server for you.

denyfirst's own promises hold for Mohur as for everything it publishes. They are at [denyfirst.dev/privacy](https://denyfirst.dev/privacy). This page adds Mohur's own, which only narrow them, and says what each part keeps.

## Who holds what

- **Your organization** runs the server you joined. Its admin decides who is on it and whom you may write to, and your organization answers for what that server keeps. To see, correct or delete what it holds about you, ask your organization's admin.
- **Partner organizations** that your admin connected run their own servers. They receive what is needed to deliver your messages to their people, and nothing more.
- **denyfirst** writes the software. It receives nothing from your app or from your organization's server.
- **Microsoft** delivers the app through the Microsoft Store, under its own privacy statement.

## What Mohur promises

Each promise says how you can check it instead of believing it.

### Servers carry only ciphertext

Messages and files are encrypted on the sender's computer with the libsignal library, unchanged, and opened only on the recipient's. No server holds message text, file contents, file names or the keys to them.

**How you check it.** The source is public. The app's end-to-end test runs two real servers, then searches their databases, stored files and logs for the message text, a file's name and its bytes, and fails if it finds any. ID: `mohur-servers-carry-only-ciphertext`.

### The app talks only to your organization's server

Apart from looking up your organization's and your partners' domains in DNS, the app connects to your organization's server and to nothing else: not to denyfirst, not to Microsoft, and not to any update or crash service.

**How you check it.** Every connection the app makes is in `client/src/core/net.mjs`, to the server named in your organization's verified record. The window itself may not connect anywhere: its content security policy says `connect-src 'none'`. ID: `mohur-talks-only-to-your-server`.

### Reports carry no messages

A report goes only to your own organization's admin. It holds who sent it, whom it is about, a reason and the note you write. It never holds a message, the note is stored encrypted, and partners are not told.

**How you check it.** The server's tests `TestReportsReachOnlyTheReportersAdmin` and `TestReportNotesAreSealedAndReportsCapped`, and the app's report test. ID: `mohur-reports-carry-no-messages`.

### Blocking stays on your computer

Blocking someone changes only your app. No server is asked, and nobody is told.

**How you check it.** The app's blocking test fails if blocking or unblocking sends any request. ID: `mohur-blocking-stays-on-your-computer`.

### Crashes stay on your computer

If Mohur crashes, it writes the crash report on your computer only, never sends it, and deletes it the next time it starts. Because Mohur catches its own crashes, Windows normally has no crash report of Mohur to send.

**How you check it.** `client/src/main.mjs` starts the crash reporter with uploading turned off and clears its folder at every start. ID: `mohur-crashes-stay-on-your-computer`.

### Mail does not wait for ever

A server keeps an encrypted message until the recipient's app collects it, and at most 30 days. It keeps an encrypted file at most 30 days.

**How you check it.** The store's test `TestMailAndFilesGoAfterThirtyDays`. ID: `mohur-mail-goes-after-30-days`.

## On your computer

- The app keeps your keys, your conversations, how far you have read them and whom you blocked in one encrypted file. Its key is protected by Windows for your Windows account.
- A file someone sends you stays encrypted on your organization's server until you save it, for up to 30 days. Mohur keeps no copy of the files you save or send.
- Uninstalling Mohur removes the app. The encrypted file may remain in your AppData folder, under Mohur, until you delete it.

## What the app sends

- **To your organization's server, when you join:** the invite code, this computer's name (your admin sees it as the device name) and your public keys.
- **To your organization's server, after that:** encrypted messages, files and delivery receipts, requests for your contacts and their public keys, and the reports you choose to send.
- **To the DNS servers your computer uses:** the names it looks up, which are your organization's and your partners' domains.

## What your organization's server keeps

- About you: your user name and display name, which your admin gives you, when you joined, the device name, your public keys and your device's certificate.
- Encrypted messages and files waiting for you, with their sender, recipient, time and size.
- The reports you send.
- The partners your admin connected, and those of their people you may write to.
- A log of events, such as a device joining, a person being removed or a report arriving, with user names. It holds no message content and not the IP address of anyone who connects. Your organization decides how long the log is kept.
- In memory only: the IP addresses of recent requests, for at most eleven minutes, to limit how often anyone may call.

The server sees who writes to whom, when, and how large each message is. It never sees what is written. Your organization's admin sees the People, Partners and Reports pages. In the database, the organization's key, the admins' authenticator secrets and report notes are sealed with a key kept in a separate file, so a copy of the database alone does not reveal them.

Your admin can reset your device, so that you, or someone else, can join again as you. Everyone you write with then sees that your device changed, so it cannot happen unnoticed, and messages from before stay on the old device.

## What a partner's server receives

Only if your admin lets you write with that partner: your address, your display name, your device's public keys and certificate, and the encrypted messages and files you send to their people, with sender, recipient, time and size. A partner never hears of your reports or whom you block. What a partner's server keeps is up to that organization.

## Microsoft Store

- You install and update Mohur through the Microsoft Store, under Microsoft's privacy statement: [privacy.microsoft.com](https://privacy.microsoft.com/privacystatement). Mohur has no update code of its own; the Store updates it as its settings say.
- As Mohur's publisher, denyfirst sees in Microsoft Partner Center what Microsoft shows every publisher: totals of installs by country, summaries of use, the ratings and reviews people publish, and crash summaries from computers that share optional diagnostic data with Microsoft. These are totals; they do not tell us who you are. Since Mohur catches its own crashes, its crash summaries should stay empty.

## Our test organization

So that Microsoft's testers can try Mohur, denyfirst runs one Mohur organization on a server it rents in Germany. It holds only test people that we create, and keeps what any Mohur server keeps, as above. It is not for real conversations and is not offered to anyone else.

## Your rights

What your organization's server holds about you is your organization's responsibility, under the data protection law that applies to it, such as the GDPR. Ask its admin. denyfirst holds nothing about you from Mohur. If you write to us, what we keep is on [denyfirst.dev/privacy](https://denyfirst.dev/privacy).

## Contact

- About data on your organization's server: your organization's admin.
- About Mohur itself: hello@denyfirst.dev.
- Security problems: the address in [denyfirst.dev/.well-known/security.txt](https://denyfirst.dev/.well-known/security.txt).

## Changes

This page lives in Mohur's public repository, and every change to it is in [its history](https://github.com/denyfirst/mohur/commits/main/docs/policies/privacy.md).
