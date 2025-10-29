# Release Process

Bu paket **tamamen otomatik** release sistemi kullanır. Manuel bir şey yapman gerekmiyor!

## 🤖 Otomatik Release Nasıl Çalışır?

### 1. Kod Yaz ve Commit Et
```bash
git add .
git commit -m "[+] Added new feature"
git push origin main
```

### 2. CI/CD Otomatik Çalışır
```
✅ Testleri çalıştırır
✅ Build kontrol eder
✅ Testler başarılıysa:
   → Otomatik version tag oluşturur
   → GitHub Release yapar
   → pkg.go.dev günceller
```

### 3. Kullanıcılar İndirebilir
```bash
go get github.com/hasirciogluhq/migrator@latest
```

## 📝 Version Otomatiği

Commit message'ına göre otomatik version belirlenir:

### MAJOR version bump (v1.0.0 → v2.0.0)
Breaking change için:
```bash
git commit -m "[MAJOR] Breaking change: API değişti"
# veya
git commit -m "BREAKING CHANGE: API değişti"
```

### MINOR version bump (v1.0.0 → v1.1.0)  
Yeni feature için:
```bash
git commit -m "[+] Added new feature"
# veya
git commit -m "[feature] New shadow DB mode"
# veya
git commit -m "[minor] Enhanced logging"
```

### PATCH version bump (v1.0.0 → v1.0.1)
Bug fix için (default):
```bash
git commit -m "[*] Fixed migration bug"
# veya
git commit -m "Fixed typo"
# veya herhangi bir mesaj (default PATCH)
```

## 🎯 Örnekler

### Bug Fix Release
```bash
git add .
git commit -m "[*] Fixed shadow DB cleanup issue"
git push origin main

# CI/CD otomatik:
# v1.0.0 → v1.0.1
```

### Feature Release
```bash
git add .
git commit -m "[+] Added migration rollback support"
git push origin main

# CI/CD otomatik:
# v1.0.1 → v1.1.0
```

### Breaking Change Release
```bash
git add .
git commit -m "[MAJOR] Changed Migrate() signature
[*] Now requires context as first parameter"
git push origin main

# CI/CD otomatik:
# v1.1.0 → v2.0.0
```

## 🔄 Pull Request Flow

### PR Oluşturulunca
```
✅ Testler otomatik çalışır
❌ Release OLMAZ (sadece test)
```

### PR Merge Edilince
```
✅ Testler tekrar çalışır
✅ Başarılıysa → OTOMATIK RELEASE!
```

## ⚠️ Testler Başarısız Olursa?

```bash
git push origin main

# CI/CD:
✅ Testleri çalıştırır
❌ Testler fail olur
⛔ RELEASE YAPILMAZ
💡 Commit main'de kalır ama release olmaz
```

**Ne yapmalısın:**
1. Sorunu düzelt
2. Yeni commit at
3. Push et
4. CI/CD tekrar dener

## 📊 Release Timeline

```
Commit → Push
   ↓
CI/CD Başlar (1-2dk)
   ↓
Testler Çalışır (30sn)
   ↓
   ├─ ✅ Başarılı
   │     ↓
   │  Version Hesapla (commit message'dan)
   │     ↓
   │  Tag Oluştur (otomatik)
   │     ↓
   │  GitHub Release (otomatik)
   │     ↓
   │  pkg.go.dev Güncelle (otomatik)
   │     ↓
   │  ✅ DONE! Herkes kullanabilir
   │
   └─ ❌ Başarısız
        ↓
     Release YOK (güvenli)
```

## 🎬 İlk Release

İlk commit'te `v0.1.0` olarak başlar:

```bash
# İlk commit ve push
git push origin main

# Otomatik: v0.1.0 release
```

## 📱 Release Takibi

### GitHub'da İzle
- **Releases**: `https://github.com/hasirciogluhq/migrator/releases`
- **Actions**: `https://github.com/hasirciogluhq/migrator/actions`
- Her commit'te CI/CD durumunu görebilirsin

### pkg.go.dev'de İzle
- `https://pkg.go.dev/github.com/hasirciogluhq/migrator`
- Tüm versiyonlar otomatik listelenir

## 🚫 Yapma Listesi

❌ Manuel tag atma - otomatik  
❌ Manuel release oluşturma - otomatik  
❌ Version number belirleme - otomatik  
❌ Changelog yazma - otomatik  
❌ pkg.go.dev güncelleme - otomatik  

## ✅ Yapma Listesi

✅ Kod yaz  
✅ Test et (`make test-docker`)  
✅ Commit et (doğru message formatı)  
✅ Push et  
✅ GitHub Actions'ı izle (optional)  

## 💡 Pro Tips

1. **Commit message önemli!** 
   - `[+]` = minor bump
   - `[*]` = patch bump
   - `[MAJOR]` = major bump

2. **Main'e push = Release**
   - Her main push potansiyel release
   - Test başarısız = güvende (release olmaz)

3. **PR'da test, merge'de release**
   - PR açtığında sadece test
   - Merge ettiğinde otomatik release

4. **Hotfix için**
   ```bash
   git commit -m "[*] Critical bug fix"
   git push origin main
   # Otomatik patch version (v1.0.0 → v1.0.1)
   ```

## 🔧 Sistemle Oynama

Eğer CI/CD'yi değiştirmek istersen:
- `.github/workflows/ci-cd.yml` dosyasını düzenle
- Test logic'i değiştirebilirsin
- Version rules'ları özelleştirebilirsin

## ❓ SSS

**S: Manuel release yapabilir miyim?**  
C: Evet ama gerek yok. Ama illa istersen: `git tag v1.0.0 && git push origin v1.0.0`

**S: Release'i geri alabilir miyim?**  
C: Evet, GitHub'dan release'i sil. Ama tag kalacak, kullanıcılar yine indirebilir.

**S: Testler geçmeden release olabilir mi?**  
C: Hayır, imkansız. CI/CD engeller.

**S: Her commit'te release oluyor mu?**  
C: Evet! Main'e her push'ta (testler başarılıysa). Dikkatli commit at!

**S: PR'da release olur mu?**  
C: Hayır, sadece testler çalışır. Merge edilince release olur.

---

**Özet:** Kod yaz, push et, gerisini CI/CD halleder! 🚀
