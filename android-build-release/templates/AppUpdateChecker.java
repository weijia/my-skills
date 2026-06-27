package com.stock.app.util;

import android.app.Activity;
import android.app.AlertDialog;
import android.app.DownloadManager;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.DialogInterface;
import android.content.Intent;
import android.content.IntentFilter;
import android.content.pm.PackageInfo;
import android.content.pm.PackageManager;
import android.database.Cursor;
import android.net.Uri;
import android.os.Build;
import android.os.Environment;
import android.os.FileProvider;
import android.util.Log;
import android.widget.Toast;

import org.json.JSONObject;

import java.io.File;
import java.net.HttpURLConnection;
import java.net.URL;

/**
 * APP 更新检查器
 *
 * 功能：
 * - 检查服务器上是否有新版本 APK
 * - 下载 APK 到本地
 * - 触发系统安装（兼容 Android 4.0 ~ Android 14+）
 *
 * 使用方法：
 * AppUpdateChecker.checkForUpdate(activity, "https://server/apps/stock-app/version.json");
 *
 * version.json 格式：
 * {
 *   "versionCode": 128,
 *   "versionName": "1.0.0",
 *   "downloadUrl": "https://server/apps/stock-app/stock-app-v1.0.0-build128.apk",
 *   "changelog": "修复了若干问题"
 * }
 */
public class AppUpdateChecker {

    private static final String TAG = "AppUpdateChecker";
    private static final String PREF_NAME = "stock_update";
    private static final String KEY_IGNORE_VERSION = "ignore_version";

    /**
     * 检查更新（异步）
     *
     * @param activity   当前 Activity
     * @param versionUrl version.json 的 URL
     */
    public static void checkForUpdate(final Activity activity, final String versionUrl) {
        new Thread(new Runnable() {
            @Override
            public void run() {
                try {
                    URL url = new URL(versionUrl);
                    HttpURLConnection conn = (HttpURLConnection) url.openConnection();
                    conn.setConnectTimeout(10000);
                    conn.setReadTimeout(10000);
                    conn.setRequestMethod("GET");

                    int responseCode = conn.getResponseCode();
                    if (responseCode != 200) {
                        Log.w(TAG, "检查更新失败: HTTP " + responseCode);
                        return;
                    }

                    java.io.BufferedReader reader = new java.io.BufferedReader(
                        new java.io.InputStreamReader(conn.getInputStream(), "UTF-8"));
                    StringBuilder sb = new StringBuilder();
                    String line;
                    while ((line = reader.readLine()) != null) {
                        sb.append(line);
                    }
                    reader.close();

                    final JSONObject versionInfo = new JSONObject(sb.toString());
                    final int serverVersionCode = versionInfo.getInt("versionCode");
                    final String serverVersionName = versionInfo.getString("versionName");
                    final String downloadUrl = versionInfo.getString("downloadUrl");
                    final String changelog = versionInfo.optString("changelog", "");

                    // 获取本地版本
                    PackageInfo pkgInfo = activity.getPackageManager()
                        .getPackageInfo(activity.getPackageName(), 0);
                    final int localVersionCode = pkgInfo.versionCode;
                    final String localVersionName = pkgInfo.versionName;

                    Log.d(TAG, "本地版本: " + localVersionName + " (" + localVersionCode + ")");
                    Log.d(TAG, "服务器版本: " + serverVersionName + " (" + serverVersionCode + ")");

                    if (serverVersionCode <= localVersionCode) {
                        Log.d(TAG, "已是最新版本");
                        return;
                    }

                    // 检查用户是否忽略过此版本
                    if (isVersionIgnored(activity, serverVersionCode)) {
                        Log.d(TAG, "用户已忽略版本 " + serverVersionCode);
                        return;
                    }

                    // 在主线程显示更新对话框
                    activity.runOnUiThread(new Runnable() {
                        @Override
                        public void run() {
                            showUpdateDialog(activity, serverVersionCode,
                                serverVersionName, localVersionName,
                                downloadUrl, changelog);
                        }
                    });

                } catch (final Exception e) {
                    Log.e(TAG, "检查更新异常: " + e.getMessage());
                }
            }
        }).start();
    }

    /**
     * 显示更新对话框
     */
    private static void showUpdateDialog(final Activity activity,
                                          final int newVersionCode,
                                          final String newVersionName,
                                          final String currentVersionName,
                                          final String downloadUrl,
                                          final String changelog) {

        StringBuilder message = new StringBuilder();
        message.append("当前版本: ").append(currentVersionName).append("\n");
        message.append("最新版本: ").append(newVersionName).append("\n");
        if (!changelog.isEmpty()) {
            message.append("\n更新内容:\n").append(changelog);
        }

        new AlertDialog.Builder(activity)
            .setTitle("发现新版本")
            .setMessage(message.toString())
            .setPositiveButton("立即更新", new DialogInterface.OnClickListener() {
                @Override
                public void onClick(DialogInterface dialog, int which) {
                    downloadAndInstall(activity, newVersionCode, downloadUrl);
                }
            })
            .setNegativeButton("稍后提醒", null)
            .setNeutralButton("忽略此版本", new DialogInterface.OnClickListener() {
                @Override
                public void onClick(DialogInterface dialog, int which) {
                    setVersionIgnored(activity, newVersionCode);
                    Toast.makeText(activity, "已忽略版本 " + newVersionName,
                        Toast.LENGTH_SHORT).show();
                }
            })
            .setCancelable(false)
            .show();
    }

    /**
     * 下载并安装 APK
     */
    private static void downloadAndInstall(final Activity activity,
                                            final int versionCode,
                                            final String downloadUrl) {
        try {
            String fileName = "stock-app-v" + versionCode + ".apk";
            File outputFile;

            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
                // Android 10+ 使用应用专属目录
                outputFile = new File(activity.getExternalFilesDir(Environment.DIRECTORY_DOWNLOADS), fileName);
            } else {
                // Android 9 及以下使用公共 Downloads 目录
                outputFile = new File(Environment.getExternalStoragePublicDirectory(
                    Environment.DIRECTORY_DOWNLOADS), fileName);
            }

            // 使用 DownloadManager 下载
            DownloadManager.Request request = new DownloadManager.Request(Uri.parse(downloadUrl));
            request.setTitle("正在下载更新...");
            request.setDescription("stock-app v" + versionCode);
            request.setDestinationUri(Uri.fromFile(outputFile));
            request.setNotificationVisibility(
                DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED);

            final DownloadManager dm = (DownloadManager) activity.getSystemService(Context.DOWNLOAD_SERVICE);
            final long downloadId = dm.enqueue(request);

            // 监听下载完成
            BroadcastReceiver receiver = new BroadcastReceiver() {
                @Override
                public void onReceive(Context context, Intent intent) {
                    long id = intent.getLongExtra(DownloadManager.EXTRA_DOWNLOAD_ID, -1);
                    if (id != downloadId) return;

                    // 检查下载状态
                    DownloadManager.Query query = new DownloadManager.Query();
                    query.setFilterById(downloadId);
                    Cursor cursor = null;
                    try {
                        cursor = dm.query(query);
                        if (cursor.moveToFirst()) {
                            int status = cursor.getInt(
                                cursor.getColumnIndexOrThrow(DownloadManager.COLUMN_STATUS));
                            if (status == DownloadManager.STATUS_SUCCESSFUL) {
                                installApk(activity, outputFile);
                            } else {
                                int reason = cursor.getInt(
                                    cursor.getColumnIndexOrThrow(DownloadManager.COLUMN_REASON));
                                Toast.makeText(activity,
                                    "下载失败 (reason=" + reason + ")", Toast.LENGTH_LONG).show();
                            }
                        }
                    } finally {
                        if (cursor != null) cursor.close();
                    }
                    activity.unregisterReceiver(this);
                }
            };

            activity.registerReceiver(receiver,
                new IntentFilter(DownloadManager.ACTION_DOWNLOAD_COMPLETE));

            Toast.makeText(activity, "开始下载更新...", Toast.LENGTH_SHORT).show();

        } catch (Exception e) {
            Log.e(TAG, "下载失败: " + e.getMessage());
            Toast.makeText(activity, "下载失败: " + e.getMessage(), Toast.LENGTH_LONG).show();
        }
    }

    /**
     * 安装 APK
     * 兼容 Android 4.0 ~ Android 14+
     */
    private static void installApk(Activity activity, File apkFile) {
        try {
            if (!apkFile.exists()) {
                Toast.makeText(activity, "APK 文件不存在", Toast.LENGTH_SHORT).show();
                return;
            }

            Intent intent = new Intent(Intent.ACTION_VIEW);
            intent.setFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION | Intent.FLAG_ACTIVITY_NEW_TASK);
            Uri apkUri;

            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) {
                // Android 7.0+ 需要 FileProvider
                apkUri = FileProvider.getUriForFile(activity,
                    activity.getPackageName() + ".fileprovider", apkFile);
            } else {
                apkUri = Uri.fromFile(apkFile);
            }

            intent.setDataAndType(apkUri, "application/vnd.android.package-archive");
            activity.startActivity(intent);

        } catch (Exception e) {
            Log.e(TAG, "安装失败: " + e.getMessage());
            Toast.makeText(activity, "安装失败: " + e.getMessage(), Toast.LENGTH_LONG).show();
        }
    }

    private static boolean isVersionIgnored(Context context, int versionCode) {
        return context.getSharedPreferences(PREF_NAME, Context.MODE_PRIVATE)
            .getInt(KEY_IGNORE_VERSION, -1) == versionCode;
    }

    private static void setVersionIgnored(Context context, int versionCode) {
        context.getSharedPreferences(PREF_NAME, Context.MODE_PRIVATE)
            .edit().putInt(KEY_IGNORE_VERSION, versionCode).apply();
    }
}
