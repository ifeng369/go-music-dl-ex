plugins {
    id("com.android.application")
}

android {
    namespace = "com.musicdl.shell"
    compileSdk = 34

    defaultConfig {
        applicationId = "com.musicdl.shell"
        minSdk = 21
        targetSdk = 34
        versionCode = 1
        versionName = "1.0.0"
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
