/**
 * Google Analytics 4 (GA4) Analytics Wrapper for gha-local-lint
 * Measurement ID: gha-local-lint
 */

(function () {
  'use strict';

  var MEASUREMENT_ID = 'gha-local-lint';

  function isGtagAvailable() {
    return typeof window !== 'undefined' && typeof window.gtag === 'function';
  }

  function trackEvent(eventName, params) {
    try {
      if (!isGtagAvailable()) {
        return;
      }
      var payload = params || {};
      payload.send_to = MEASUREMENT_ID;
      window.gtag('event', eventName, payload);
    } catch (err) {
      // Suppress analytics errors to prevent page disruption
      if (console && console.debug) {
        console.debug('Analytics event failed:', err);
      }
    }
  }

  function trackPageView(pagePath) {
    trackEvent('page_view', {
      page_path: pagePath || window.location.pathname,
      page_title: document.title,
      page_location: window.location.href
    });
  }

  function trackCopyCommand(commandName) {
    trackEvent('copy_install_command', {
      command: commandName || 'brew install gha-local-lint'
    });
  }

  function trackDownloadClick(platform) {
    trackEvent('download_binary', {
      platform: platform || 'unknown'
    });
  }

  // Export to global scope
  window.GhaAnalytics = {
    trackEvent: trackEvent,
    trackPageView: trackPageView,
    trackCopyCommand: trackCopyCommand,
    trackDownloadClick: trackDownloadClick
  };

  // Initial page view track on load
  if (document.readyState === 'complete' || document.readyState === 'interactive') {
    trackPageView();
  } else {
    document.addEventListener('DOMContentLoaded', function () {
      trackPageView();
    });
  }
})();
