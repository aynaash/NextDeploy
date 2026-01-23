# NextDeploy Webi Installers - Quick Reference

## ✅ What's Done

Created webi installers for both NextDeploy CLI and daemon:

```
webi-installers/
├── nextdeploy/          ✅ CLI installer (Windows, macOS, Linux)
│   ├── README.md        ✅ Package description with cheat sheet
│   ├── releases.js      ✅ GitHub releases API integration
│   ├── install.sh       ✅ POSIX shell installer
│   └── install.ps1      ✅ PowerShell installer (Windows)
│
├── nextdeployd/         ✅ Daemon installer (Linux only)
│   ├── README.md        ✅ Package description
│   ├── releases.js      ✅ GitHub releases API (Linux-filtered)
│   └── install.sh       ✅ POSIX installer + systemd service
│
└── SUBMISSION_GUIDE.md  ✅ How to test and submit
```

## 🚀 Next Steps

### 1. Test Locally (Before Submitting)

```sh
# Clone webi-installers repo
git clone https://github.com/webinstall/webi-installers.git
cd webi-installers
git submodule update --init
npm clean-install

# Copy your installers
cp -r ../NextDeploy/webi-installers/nextdeploy ./
cp -r ../NextDeploy/webi-installers/nextdeployd ./

# Test them
node _webi/test.js ./nextdeploy/
node _webi/test.js ./nextdeployd/
```

### 2. Create a GitHub Release

Your GoReleaser is already configured! Just:

```sh
# Tag a release
git tag v0.1.0
git push origin v0.1.0

# GitHub Actions will automatically:
# - Build binaries for all platforms
# - Create GitHub release
# - Upload binaries with correct names
```

**Binary names will be**:
- `nextdeploy-linux-amd64`
- `nextdeploy-darwin-arm64`
- `nextdeploy-windows-amd64.exe`
- `nextdeployd-linux-amd64`
- etc.

### 3. Submit to Webi

```sh
# Fork the repo
gh repo fork webinstall/webi-installers

# Add your installers
cp -r webi-installers/nextdeploy webi-installers-fork/
cp -r webi-installers/nextdeployd webi-installers-fork/

# Create PR
cd webi-installers-fork
git checkout -b add-nextdeploy
git add nextdeploy nextdeployd
git commit -m "feat: add nextdeploy and nextdeployd installers"
git push origin add-nextdeploy
gh pr create --title "feat: add nextdeploy and nextdeployd" --body "Adds installers for NextDeploy CLI and daemon"
```

## 📝 After Approval

Users can install with one command:

```sh
# CLI (all platforms)
curl https://webi.sh/nextdeploy | sh

# Daemon (Linux servers)
curl https://webi.sh/nextdeployd | sh
```

## 🎯 For Your Blog Post

You can now say:

> **One-Line Installation**
> 
> NextDeploy is available via webi for instant installation on any platform:
> 
> ```sh
> curl https://webi.sh/nextdeploy | sh
> ```
> 
> No sudo, no package manager, no system dependencies. Just works.

## 🔍 Checklist

- [x] Create webi installer structure
- [x] Write README with cheat sheets
- [x] Create releases.js (GitHub API)
- [x] Write install.sh (POSIX)
- [x] Write install.ps1 (Windows)
- [x] Add systemd service for daemon
- [ ] Test locally with webi test suite
- [ ] Create v0.1.0 release on GitHub
- [ ] Submit PR to webi-installers
- [ ] Wait for approval
- [ ] Update blog post with install command

## 💡 Pro Tips

1. **Test before submitting**: The webi test suite catches most issues
2. **Keep READMEs updated**: They become your official docs on webi.sh
3. **Use semantic versioning**: v0.1.0, v0.2.0, etc.
4. **Tag releases properly**: Must start with 'v'
5. **Check binary names**: Must match the pattern webi expects

## 📚 Resources

- **Webi Homepage**: https://webinstall.dev
- **Webi Repo**: https://github.com/webinstall/webi-installers
- **Your Installers**: `./webi-installers/`
- **Submission Guide**: `./webi-installers/SUBMISSION_GUIDE.md`
