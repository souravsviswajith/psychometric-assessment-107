package com.career107.app;

import android.app.Activity;
import android.content.Intent;
import android.graphics.Color;
import android.net.Uri;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.print.PrintAttributes;
import android.print.PrintManager;
import android.view.Gravity;
import android.view.View;
import android.webkit.JavascriptInterface;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.TextView;

import com.career107.mobile.Mobile;

/**
 * Hosts the Career Assessment 107 UI.
 *
 * The Go runtime (career107.aar) starts an HTTP server on the loopback
 * interface inside this process and serves both the API and the UI. This
 * activity only shows that UI in a WebView: all assessment logic, validation
 * and storage live in Go.
 */
public class MainActivity extends Activity {

    private WebView webView;
    private TextView status;
    private String origin;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);

        status = new TextView(this);
        status.setText(R.string.starting);
        status.setTextColor(Color.parseColor("#44474E"));
        status.setGravity(Gravity.CENTER);
        status.setPadding(48, 48, 48, 48);
        setContentView(status);

        // Starting the runtime replays the saved session, so it runs off the
        // UI thread.
        final String dataDir = getFilesDir().getAbsolutePath();
        new Thread(() -> {
            try {
                final String url = Mobile.start(dataDir);
                runOnUi(() -> showUi(url));
            } catch (final Exception error) {
                runOnUi(() -> status.setText(getString(R.string.start_failed, String.valueOf(error.getMessage()))));
            }
        }, "career107-start").start();
    }

    private void runOnUi(Runnable task) {
        new Handler(Looper.getMainLooper()).post(task);
    }

    private void showUi(String url) {
        origin = url;
        webView = new WebView(this);
        webView.setBackgroundColor(Color.parseColor("#F6FAFF"));
        webView.setOverScrollMode(View.OVER_SCROLL_NEVER);

        WebSettings settings = webView.getSettings();
        settings.setJavaScriptEnabled(true);
        settings.setDomStorageEnabled(true);
        settings.setAllowFileAccess(false);
        settings.setAllowContentAccess(false);
        settings.setSupportZoom(false);

        webView.setWebViewClient(new WebViewClient() {
            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest request) {
                String target = request.getUrl().toString();
                if (target.startsWith(origin)) {
                    return false;
                }
                // Anything outside the app opens in the system browser.
                startActivity(new Intent(Intent.ACTION_VIEW, Uri.parse(target)));
                return true;
            }
        });

        // "Download Dossier (PDF)" prints the page through Android's print
        // service, which offers Save as PDF.
        webView.addJavascriptInterface(new Object() {
            @JavascriptInterface
            public void print() {
                runOnUi(() -> {
                    PrintManager printManager = (PrintManager) getSystemService(PRINT_SERVICE);
                    printManager.print("Career Dossier", webView.createPrintDocumentAdapter("Career Dossier"),
                            new PrintAttributes.Builder().build());
                });
            }
        }, "Career107Native");

        setContentView(webView);
        webView.loadUrl(url);
    }

    @Override
    @SuppressWarnings("deprecation")
    public void onBackPressed() {
        if (webView == null) {
            super.onBackPressed();
            return;
        }
        // The app's screens are not browser history entries, so the page decides
        // whether Back steps back inside the app or leaves it.
        webView.evaluateJavascript(
                "(window.__career107Back ? window.__career107Back() : false)",
                handled -> {
                    if (!"true".equals(handled)) {
                        finish();
                    }
                });
    }

    @Override
    protected void onDestroy() {
        if (webView != null) {
            webView.destroy();
        }
        super.onDestroy();
    }
}
