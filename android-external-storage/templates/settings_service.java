package com.example.your_app.util;

import android.app.Activity;
import android.content.ContentResolver;
import android.content.Context;
import android.content.Intent;
import android.content.SharedPreferences;
import android.database.Cursor;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.provider.DocumentsContract;
import android.util.Log;

import org.json.JSONException;
import org.json.JSONObject;

import java.io.BufferedReader;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.InputStream;
import java.io.InputStreamReader;
import java.io.OutputStream;

/**
 * 外部存储配置管理器（纯 Java Android 版本）
 * 
 * 支持两种存储方式：
 * 1. SAF (Storage Access Framework) - Android 4.4+ (API 19+)
 *    配置保存到用户选择的外部目录，卸载后保留
 * 2. 外部存储直接写入 - Android 4.0-4.3 (API 14-18)
 *    配置保存到公共目录 /sdcard/YourApp/
 * 
 * 同时备份到 SharedPreferences 作为降级方案
 */
public class ExternalStorageManager {
    private static final String TAG = "ExternalStorage";
    private static final String PREF_NAME = "app_config";
    private static final String KEY_CONFIG = "config_json";
    private static final String KEY_SAF_URI = "saf_dir_uri";
    private static final String CONFIG_FILE_NAME = "settings.json";
    
    // 外部存储公共目录（用于 Android 4.0-4.3）
    private static final String LEGACY_CONFIG_DIR = "YourApp";  // 替换为你的应用名
    
    // SAF 请求码
    public static final int SAF_REQUEST_CODE = 1001;
    
    private Context context;
    private SharedPreferences prefs;
    private Uri safDirUri;
    private boolean safAvailable;
    
    public ExternalStorageManager(Context context) {
        this.context = context;
        this.prefs = context.getSharedPreferences(PREF_NAME, Context.MODE_PRIVATE);
        
        // 检查 SAF 是否可用
        safAvailable = Build.VERSION.SDK_INT >= Build.VERSION_CODES.KITKAT; // API 19
        
        // 恢复 SAF 目录 URI
        restoreSafDirectory();
    }
    
    /**
     * 检查 SAF 是否可用
     */
    public boolean isSafAvailable() {
        return safAvailable;
    }
    
    /**
     * 检查是否有 SAF 目录
     */
    public boolean hasSafDirectory() {
        return safDirUri != null;
    }
    
    /**
     * 请求用户选择 SAF 目录
     */
    public void requestSafDirectory(Activity activity) {
        if (!safAvailable) return;
        
        Intent intent = new Intent(Intent.ACTION_OPEN_DOCUMENT_TREE);
        intent.addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION 
            | Intent.FLAG_GRANT_WRITE_URI_PERMISSION 
            | Intent.FLAG_GRANT_PERSISTABLE_URI_PERMISSION 
            | Intent.FLAG_GRANT_PREFIX_URI_PERMISSION);
        
        activity.startActivityForResult(intent, SAF_REQUEST_CODE);
    }
    
    /**
     * 处理 SAF 目录选择结果
     */
    public boolean handleSafResult(int requestCode, int resultCode, Intent data) {
        if (requestCode != SAF_REQUEST_CODE || resultCode != Activity.RESULT_OK) {
            return false;
        }
        
        if (data == null || data.getData() == null) return false;
        
        Uri uri = data.getData();
        
        ContentResolver resolver = context.getContentResolver();
        resolver.takePersistableUriPermission(
            uri,
            Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_GRANT_WRITE_URI_PERMISSION
        );
        
        safDirUri = uri;
        prefs.edit().putString(KEY_SAF_URI, uri.toString()).apply();
        
        return true;
    }
    
    /**
     * 恢复 SAF 目录
     */
    public void restoreSafDirectory() {
        String uriString = prefs.getString(KEY_SAF_URI, null);
        if (uriString != null) {
            try {
                safDirUri = Uri.parse(uriString);
            } catch (Exception e) {
                safDirUri = null;
            }
        }
    }
    
    /**
     * 加载配置（优先级：SAF > 外部存储 > SharedPreferences）
     */
    public JSONObject loadConfig() {
        JSONObject config = null;
        
        if (safAvailable && safDirUri != null) {
            config = loadFromSaf();
            if (config != null) return config;
        }
        
        config = loadFromExternalStorage();
        if (config != null) return config;
        
        return loadFromSharedPreferences();
    }
    
    /**
     * 保存配置（同时保存到所有位置）
     */
    public void saveConfig(JSONObject config) {
        String content = config.toString();
        
        if (safAvailable && safDirUri != null) {
            try { saveToSaf(content); } catch (Exception e) { Log.e(TAG, e.getMessage()); }
        }
        
        try { saveToExternalStorage(content); } catch (Exception e) { Log.e(TAG, e.getMessage()); }
        
        prefs.edit().putString(KEY_CONFIG, content).apply();
    }
    
    // SAF 操作实现（略，参考完整版本）
    private JSONObject loadFromSaf() { /* ... */ return null; }
    private void saveToSaf(String content) throws Exception { /* ... */ }
    
    // 外部存储操作
    private JSONObject loadFromExternalStorage() {
        try {
            File configDir = getLegacyConfigDir();
            if (configDir == null) return null;
            
            File configFile = new File(configDir, CONFIG_FILE_NAME);
            if (!configFile.exists()) return null;
            
            FileInputStream fis = new FileInputStream(configFile);
            BufferedReader reader = new BufferedReader(new InputStreamReader(fis, "UTF-8"));
            StringBuilder sb = new StringBuilder();
            String line;
            while ((line = reader.readLine()) != null) sb.append(line);
            reader.close();
            
            return new JSONObject(sb.toString());
        } catch (Exception e) { return null; }
    }
    
    private void saveToExternalStorage(String content) throws Exception {
        File configDir = getLegacyConfigDir();
        if (configDir == null) throw new Exception("无法访问外部存储");
        
        if (!configDir.exists()) configDir.mkdirs();
        
        File configFile = new File(configDir, CONFIG_FILE_NAME);
        FileOutputStream fos = new FileOutputStream(configFile);
        fos.write(content.getBytes("UTF-8"));
        fos.close();
    }
    
    private File getLegacyConfigDir() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            File externalDir = context.getExternalFilesDir(null);
            if (externalDir != null) return new File(externalDir, LEGACY_CONFIG_DIR);
        }
        
        File storageDir = Environment.getExternalStorageDirectory();
        if (storageDir != null && storageDir.canWrite()) {
            return new File(storageDir, LEGACY_CONFIG_DIR);
        }
        
        return null;
    }
    
    private JSONObject loadFromSharedPreferences() {
        String configJson = prefs.getString(KEY_CONFIG, null);
        if (configJson == null) return new JSONObject();
        try { return new JSONObject(configJson); } 
        catch (JSONException e) { return new JSONObject(); }
    }
    
    public String getStorageLocation() {
        if (safAvailable && safDirUri != null) return "SAF: " + safDirUri.toString();
        File configDir = getLegacyConfigDir();
        if (configDir != null) return "外部存储: " + configDir.getAbsolutePath();
        return "SharedPreferences";
    }
}