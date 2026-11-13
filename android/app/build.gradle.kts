plugins {
    id("com.android.application")
}

// The release workflow injects the tag version so every APK reports a build
// number that matches the release it was published with.
val appVersion = System.getenv("APP_VERSION") ?: "1.0.0"
val versionParts = appVersion.split(".").mapNotNull { it.toIntOrNull() }

android {
    namespace = "com.musicdl.shell"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.musicdl.shell"
        minSdk = 21
        targetSdk = 34
        versionCode = versionParts.getOrElse(0) { 1 } * 10000 +
            versionParts.getOrElse(1) { 0 } * 100 +
            versionParts.getOrElse(2) { 0 }
        versionName = appVersion
    }

    signingConfigs {
        // Fixed sideload key committed in the repository so every release can
        // overwrite the previous install. A per-machine debug key changes on
        // each CI run, which made upgrades impossible.
        create("release") {
            val store = rootProject.file("keystore/musicdl-release.p12")
            if (store.exists()) {
                storeFile = store
                storeType = "PKCS12"
                storePassword = "musicdl2024"
                keyAlias = "musicdl"
                keyPassword = "musicdl2024"
            }
        }
    }

    buildFeatures {
        buildConfig = true
    }

    flavorDimensions += "mode"
    productFlavors {
        // Lightweight shell that talks to a server the user enters.
        create("remote") {
            dimension = "mode"
        }
        // Pocket server: the Go core is embedded, no external dependency.
        create("standalone") {
            dimension = "mode"
            applicationIdSuffix = ".standalone"
            versionNameSuffix = "-standalone"
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            val fixed = signingConfigs.findByName("release")
            signingConfig = if (fixed != null && fixed.storeFile != null) {
                fixed
            } else {
                // Local builds without the keystore still produce an APK.
                signingConfigs.getByName("debug")
            }
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    lint {
        abortOnError = false
    }
}

dependencies {
    // Produced by `gomobile bind` from src/server/mobile. Only the standalone
    // flavour embeds it, so the remote build stays tiny.
    val mobileAar = file("libs/mobile.aar")
    if (mobileAar.exists()) {
        add("standaloneImplementation", files(mobileAar))
    }
}
