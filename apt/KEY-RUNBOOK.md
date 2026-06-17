# ekiben APT repo — one-time setup runbook

These steps are performed **once**, locally, by the maintainer. The private key is
never committed; it lives only as the encrypted GitHub Actions secret `GPG_PRIVATE_KEY`.

## 1. Generate the dedicated signing key (RSA 4096, no passphrase)

```bash
cat > /tmp/ekiben-key.conf <<'EOF'
Key-Type: RSA
Key-Length: 4096
Name-Real: ekiben repository signing
Name-Email: kevin.froster.personal@outlook.it
Expire-Date: 0
%no-protection
%commit
EOF
gpg --batch --gen-key /tmp/ekiben-key.conf
shred -u /tmp/ekiben-key.conf
gpg --list-keys "ekiben repository signing"
```

## 2. Store the PRIVATE key as a GitHub Actions secret

```bash
gpg --armor --export-secret-keys "ekiben repository signing" | gh secret set GPG_PRIVATE_KEY --repo KewinGit/ekiben
```

## 3. Create an empty gh-pages branch (so CI can check it out)

```bash
git switch --orphan gh-pages
git commit --allow-empty -m "ADD: inizializza branch gh-pages per APT repo"
git push -u origin gh-pages
git switch main
```

## 4. Enable GitHub Pages from gh-pages

```bash
gh api -X POST repos/KewinGit/ekiben/pages -f 'source[branch]=gh-pages' -f 'source[path]=/' || \
gh api -X PUT  repos/KewinGit/ekiben/pages -f 'source[branch]=gh-pages' -f 'source[path]=/'
```
(Or: repo Settings → Pages → Source: Deploy from a branch → `gh-pages` / root.)

## Key rotation / revocation

The key is dedicated to the repo, so compromise is contained: generate a new key
(steps 1–2), delete the old `ekiben.gpg` from `gh-pages`, re-run a release to
re-sign `dists/` and rebuild the keyring `.deb`, and notify users to reinstall the
keyring package.
