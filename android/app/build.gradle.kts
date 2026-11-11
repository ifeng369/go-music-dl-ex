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

    buildTypes {
        release {
            isMinifyEnabled = false
            // Sideload build: signed with the standard debug key so the APK can
            // be installed without publishing a private keystore.
            signingConfig = signingConfigs.getByName("debug")
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
