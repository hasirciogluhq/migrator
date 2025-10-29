# Release Process

This package uses a **fully automated** release system. No manual steps required!

## 🤖 How Automated Releases Work

### 1. Write Code and Commit
```bash
git add .
git commit -m "[+] Added new feature"
git push origin main
```

### 2. CI/CD Runs Automatically
```
✅ Runs tests
✅ Checks build
✅ If tests pass:
   → Automatically creates version tag
   → Creates GitHub Release
   → Updates pkg.go.dev
```

### 3. Users Can Install
```bash
go get github.com/hasirciogluhq/migrator@latest
```

## 📝 Automatic Versioning

Version is automatically determined from commit message:

### MAJOR version bump (v1.0.0 → v2.0.0)
For breaking changes:
```bash
git commit -m "[MAJOR] Breaking change: API changed"
# or
git commit -m "BREAKING CHANGE: API changed"
```

### MINOR version bump (v1.0.0 → v1.1.0)  
For new features:
```bash
git commit -m "[+] Added new feature"
# or
git commit -m "[feature] New shadow DB mode"
# or
git commit -m "[minor] Enhanced logging"
```

### PATCH version bump (v1.0.0 → v1.0.1)
For bug fixes (default):
```bash
git commit -m "[*] Fixed migration bug"
# or
git commit -m "Fixed typo"
# or any message (defaults to PATCH)
```

## 🎯 Examples

### Bug Fix Release
```bash
git add .
git commit -m "[*] Fixed shadow DB cleanup issue"
git push origin main

# CI/CD automatically:
# v1.0.0 → v1.0.1
```

### Feature Release
```bash
git add .
git commit -m "[+] Added migration rollback support"
git push origin main

# CI/CD automatically:
# v1.0.1 → v1.1.0
```

### Breaking Change Release
```bash
git add .
git commit -m "[MAJOR] Changed Migrate() signature
[*] Now requires context as first parameter"
git push origin main

# CI/CD automatically:
# v1.1.0 → v2.0.0
```

## 🔄 Pull Request Flow

### When PR is Created
```
✅ Tests run automatically
❌ NO release (tests only)
```

### When PR is Merged
```
✅ Tests run again
✅ If successful → AUTOMATIC RELEASE!
```

## ⚠️ If Tests Fail

```bash
git push origin main

# CI/CD:
✅ Runs tests
❌ Tests fail
⛔ NO RELEASE
💡 Commit stays on main but no release
```

**What to do:**
1. Fix the issue
2. Make new commit
3. Push again
4. CI/CD tries again

## 📊 Release Timeline

```
Commit → Push
   ↓
CI/CD Starts (1-2min)
   ↓
Tests Run (30sec)
   ↓
   ├─ ✅ Success
   │     ↓
   │  Calculate Version (from commit message)
   │     ↓
   │  Create Tag (automatic)
   │     ↓
   │  GitHub Release (automatic)
   │     ↓
   │  Update pkg.go.dev (automatic)
   │     ↓
   │  ✅ DONE! Everyone can use it
   │
   └─ ❌ Failed
        ↓
     NO Release (safe)
```

## 🎬 First Release

First commit starts as `v0.1.0`:

```bash
# First commit and push
git push origin main

# Automatically: v0.1.0 release
```

## 📱 Track Releases

### On GitHub
- **Releases**: `https://github.com/hasirciogluhq/migrator/releases`
- **Actions**: `https://github.com/hasirciogluhq/migrator/actions`
- See CI/CD status for each commit

### On pkg.go.dev
- `https://pkg.go.dev/github.com/hasirciogluhq/migrator`
- All versions automatically listed

## 🚫 Don't Do This

❌ Manual tag creation - automatic  
❌ Manual release creation - automatic  
❌ Version number assignment - automatic  
❌ Changelog writing - automatic  
❌ pkg.go.dev updates - automatic  

## ✅ Do This

✅ Write code  
✅ Test (`make test-docker`)  
✅ Commit (correct message format)  
✅ Push  
✅ Watch GitHub Actions (optional)  

## 💡 Pro Tips

1. **Commit message matters!** 
   - `[+]` = minor bump
   - `[*]` = patch bump
   - `[MAJOR]` = major bump

2. **Push to main = Release**
   - Every main push is potential release
   - Failed tests = safe (no release)

3. **PR tests, merge releases**
   - PR opened → only tests
   - PR merged → automatic release

4. **For hotfixes**
   ```bash
   git commit -m "[*] Critical bug fix"
   git push origin main
   # Automatic patch version (v1.0.0 → v1.0.1)
   ```

## 🔧 Customize System

If you want to modify CI/CD:
- Edit `.github/workflows/ci-cd.yml`
- Change test logic
- Customize version rules

## ❓ FAQ

**Q: Can I manually release?**  
A: Yes, but not needed. But if you insist: `git tag v1.0.0 && git push origin v1.0.0`

**Q: Can I revert a release?**  
A: Yes, delete release on GitHub. But tag remains, users can still download.

**Q: Can release happen without tests passing?**  
A: No, impossible. CI/CD blocks it.

**Q: Does every commit create a release?**  
A: Yes! Every push to main (if tests pass). Commit carefully!

**Q: Do PRs create releases?**  
A: No, only tests run. Release happens when merged.

---

**Summary:** Write code, push, CI/CD handles the rest! 🚀
