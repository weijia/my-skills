// ============================================================
// Flutter Android 签名配置 - 支持可更新 APK
// ============================================================
// 问题：每次 CI 构建使用不同签名，导致无法覆盖安装
// 解决：使用固定 keystore，每次构建使用同一签名
// ============================================================

plugins {
    id("com.android.application")
    id("kotlin-android")
    id("dev.flutter.flutter-gradle-plugin")
}

android {
    namespace = "com.example.your_app"  // 替换为你的包名
    compileSdk = flutter.compileSdkVersion
    ndkVersion = flutter.ndkVersion

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    defaultConfig {
        applicationId = "com.example.your_app"  // 替换为你的包名
        minSdk = flutter.minSdkVersion
        targetSdk = flutter.targetSdkVersion
        versionCode = flutter.versionCode
        versionName = flutter.versionName
    }

    signingConfigs {
        // Release 签名配置（从环境变量读取）
        create("release") {
            val keystorePath = System.getenv("KEYSTORE_PATH")?.takeIf { it.isNotBlank() }
            val keystorePassword = System.getenv("KEYSTORE_PASSWORD")?.takeIf { it.isNotBlank() }
            val keyAlias = System.getenv("KEY_ALIAS")?.takeIf { it.isNotBlank() } ?: "release"
            val keyPassword = System.getenv("KEY_PASSWORD")?.takeIf { it.isNotBlank() }
            
            if (keystorePath != null && keystorePassword != null) {
                val keystoreFile = file(keystorePath)
                if (keystoreFile.exists()) {
                    storeFile = keystoreFile
                    storePassword = keystorePassword
                    this.keyAlias = keyAlias
                    this.keyPassword = keyPassword ?: keystorePassword
                }
            }
        }
    }

    buildTypes {
        release {
            // 如果有 release 签名配置则使用，否则使用 debug 签名
            signingConfig = if (signingConfigs.findByName("release")?.storeFile?.exists() == true) {
                signingConfigs.getByName("release")
            } else {
                signingConfigs.getByName("debug")
            }
        }
    }
}

flutter {
    source = "../.."
}

// ============================================================
// 使用说明
// ============================================================
// 
// 1. 生成 keystore（只需执行一次）：
//    keytool -genkeypair \
//      -keystore release-keystore.jks \
//      -keyalg RSA -keysize 2048 -validity 36500 \
//      -alias release \
//      -storepass your-password \
//      -keypass your-password \
//      -dname "CN=Your App, OU=Dev, O=YourOrg, C=CN"
//
// 2. 将 keystore 转为 Base64：
//    base64 -w 0 release-keystore.jks > keystore_base64.txt
//
// 3. 配置 GitHub Secrets：
//    KEYSTORE_BASE64    - keystore_base64.txt 的内容
//    KEYSTORE_PASSWORD  - keystore 密码
//    KEY_ALIAS          - key 别名（默认 release）
//    KEY_PASSWORD       - key 密码（可与 keystore 密码相同）
//
// 4. GitHub Actions 中添加：
//    - name: Setup Release Keystore
//      env:
//        KEYSTORE_BASE64: ${{ secrets.KEYSTORE_BASE64 }}
//        KEYSTORE_PASSWORD: ${{ secrets.KEYSTORE_PASSWORD }}
//        KEY_ALIAS: ${{ secrets.KEY_ALIAS }}
//        KEY_PASSWORD: ${{ secrets.KEY_PASSWORD }}
//      run: |
//        if [ -n "$KEYSTORE_BASE64" ]; then
//          echo "$KEYSTORE_BASE64" | base64 -d > android/app/release-key.jks
//        fi
//
//    - name: Build APK
//      env:
//        KEYSTORE_PATH: ${{ secrets.KEYSTORE_BASE64 != '' && 'release-key.jks' || '' }}
//        KEYSTORE_PASSWORD: ${{ secrets.KEYSTORE_PASSWORD }}
//        KEY_ALIAS: ${{ secrets.KEY_ALIAS }}
//        KEY_PASSWORD: ${{ secrets.KEY_PASSWORD }}
//      run: flutter build apk --release
//
// ============================================================
// 重要提示
// ============================================================
// 
// - 签名密钥一旦丢失，将无法更新 App，请妥善保管！
// - 所有构建必须使用相同的 keystore，否则无法覆盖安装
// - versionCode 必须递增，否则无法覆盖安装
// - applicationId 必须相同，否则会被视为不同 App
