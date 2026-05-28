    const state = {
      user: null,
      qrLoginId: null,
      qrTimer: null,
      loginMFAContext: '',
      uploadTimer: null,
      currentFolderId: '',
      folderStack: [],
      view: 'own',
      visibleFiles: [],
      serverFiles: [],
      droppedFiles: [],
      selectedFileIds: new Set(),
      selectionAnchorID: '',
      draggingItems: [],
      shareFile: null,
      shareRecipients: [],
      shareTab: 'internal',
      detailsFile: null,
      detailsDownloadActivity: null,
      detailsRequestID: 0,
      applyingRoute: false,
      uploadQueue: [],
      uploadQueueRunning: false,
      uploadMonitorRunning: false,
      nextUploadQueueID: 1,
      appInfo: null,
      uploadDebugAllowed: true,
      uploadDebugEnabled: false,
      uploadDebugLines: [],
      movePickerFolders: [],
      sharedRouteUnavailableMessage: '',
      adminInstanceID: '',
      adminLicense: null,
      mfaStatus: null,
      rememberedUser: null,
      localMFAMethods: [],
      qrExpiresAt: null,
      qrCountdownTimer: null,
      readOnlyMapMode: false,
      loginAltAccountMode: false,
      reconnectMode: false,
      phoneCountry: 'ua',
      normalizedPhone: '',
      telegramCodeRetryAt: 0,
      telegramCodeRetryTimer: null,
      telegramCodeNextType: '',
      telegramCodeCanResend: false,
      telegramCodePhone: '',
      telegramSessionStatus: '',
    };

    const el = {
      loginView: document.getElementById('loginView'),
      appView: document.getElementById('appView'),
      userbar: document.getElementById('userbar'),
      appVersion: document.getElementById('appVersion'),
      userName: document.getElementById('userName'),
      securityBtn: document.getElementById('securityBtn'),
      adminBtn: document.getElementById('adminBtn'),
      logoutForgetBtn: document.getElementById('logoutForgetBtn'),
      startQrBtn: document.getElementById('startQrBtn'),
      sendCodeBtn: document.getElementById('sendCodeBtn'),
      loginWithCodeBtn: document.getElementById('loginWithCodeBtn'),
      rememberedBox: document.getElementById('rememberedBox'),
      rememberedUser: document.getElementById('rememberedUser'),
      rememberedStatus: document.getElementById('rememberedStatus'),
      continueRememberedBtn: document.getElementById('continueRememberedBtn'),
      forgetRememberedBtn: document.getElementById('forgetRememberedBtn'),
      useAnotherAccountBtn: document.getElementById('useAnotherAccountBtn'),
      loginTelegramBox: document.getElementById('loginTelegramBox'),
      loginPhoneRow: document.getElementById('loginPhoneRow'),
      loginCountryField: document.getElementById('loginCountryField'),
      loginCountrySelect: document.getElementById('loginCountrySelect'),
      loginPhoneField: document.getElementById('loginPhoneField'),
      loginPhoneHint: document.getElementById('loginPhoneHint'),
      loginPhonePreview: document.getElementById('loginPhonePreview'),
      loginCodeField: document.getElementById('loginCodeField'),
      loginCodeLabel: document.getElementById('loginCodeLabel'),
      loginPhoneInput: document.getElementById('loginPhoneInput'),
      loginCodeInput: document.getElementById('loginCodeInput'),
      loginPasswordField: document.getElementById('loginPasswordField'),
      loginPasswordLabel: document.getElementById('loginPasswordLabel'),
      loginPasswordInput: document.getElementById('loginPasswordInput'),
      localPasswordAction: document.getElementById('localPasswordAction'),
      localPasswordField: document.getElementById('localPasswordField'),
      localPasswordInput: document.getElementById('localPasswordInput'),
      localPasswordToggleBtn: document.getElementById('localPasswordToggleBtn'),
      loginUseWebauthnBtn: document.getElementById('loginUseWebauthnBtn'),
      loginUsePasswordBtn: document.getElementById('loginUsePasswordBtn'),
      qrImage: document.getElementById('qrImage'),
      loginStatus: document.getElementById('loginStatus'),
      logoutBtn: document.getElementById('logoutBtn'),
      refreshBtn: document.getElementById('refreshBtn'),
      ownFilesBtn: document.getElementById('ownFilesBtn'),
      sharedFilesBtn: document.getElementById('sharedFilesBtn'),
      upBtn: document.getElementById('upBtn'),
      breadcrumbs: document.getElementById('breadcrumbs'),
      selectionBar: document.getElementById('selectionBar'),
      selectionSummary: document.getElementById('selectionSummary'),
      selectAllVisibleBtn: document.getElementById('selectAllVisibleBtn'),
      moveSelectedBtn: document.getElementById('moveSelectedBtn'),
      deleteSelectedBtn: document.getElementById('deleteSelectedBtn'),
      clearSelectionBtn: document.getElementById('clearSelectionBtn'),
      filesBody: document.getElementById('filesBody'),
      filePanel: document.getElementById('filePanel'),
      folderNameInput: document.getElementById('folderNameInput'),
      createFolderBtn: document.getElementById('createFolderBtn'),
      dropZone: document.getElementById('dropZone'),
      fileInput: document.getElementById('fileInput'),
      uploadBtn: document.getElementById('uploadBtn'),
      uploadStatus: document.getElementById('uploadStatus'),
      tabSafetyIndicator: document.getElementById('tabSafetyIndicator'),
      tabSafetyText: document.getElementById('tabSafetyText'),
      uploadDebugToggle: document.getElementById('uploadDebugToggle'),
      uploadDebugLog: document.getElementById('uploadDebugLog'),
      uploadQueue: document.getElementById('uploadQueue'),
      clearCompletedQueueBtn: document.getElementById('clearCompletedQueueBtn'),
      shareModal: document.getElementById('shareModal'),
      closeShareBtn: document.getElementById('closeShareBtn'),
      shareFileName: document.getElementById('shareFileName'),
      shareInternalTabBtn: document.getElementById('shareInternalTabBtn'),
      sharePublicTabBtn: document.getElementById('sharePublicTabBtn'),
      shareInternalTabPanel: document.getElementById('shareInternalTabPanel'),
      sharePublicTabPanel: document.getElementById('sharePublicTabPanel'),
      shareRecipientSelect: document.getElementById('shareRecipientSelect'),
      shareRecipientHint: document.getElementById('shareRecipientHint'),
      shareManualToggleBtn: document.getElementById('shareManualToggleBtn'),
      shareManualField: document.getElementById('shareManualField'),
      shareTelegramInput: document.getElementById('shareTelegramInput'),
      sharePermissionSelect: document.getElementById('sharePermissionSelect'),
      shareExpirySelect: document.getElementById('shareExpirySelect'),
      createShareBtn: document.getElementById('createShareBtn'),
      shareStatus: document.getElementById('shareStatus'),
      sharesBody: document.getElementById('sharesBody'),
      publicExpirySelect: document.getElementById('publicExpirySelect'),
      createPublicLinkBtn: document.getElementById('createPublicLinkBtn'),
      publicLinkResult: document.getElementById('publicLinkResult'),
      publicLinkInput: document.getElementById('publicLinkInput'),
      publicPasswordInput: document.getElementById('publicPasswordInput'),
      publicDownloadLimitInput: document.getElementById('publicDownloadLimitInput'),
      publicDownloadLimitModeSelect: document.getElementById('publicDownloadLimitModeSelect'),
      publicShowChecksumInput: document.getElementById('publicShowChecksumInput'),
      copyPublicLinkBtn: document.getElementById('copyPublicLinkBtn'),
      publicLinksBody: document.getElementById('publicLinksBody'),
      moveModal: document.getElementById('moveModal'),
      closeMoveBtn: document.getElementById('closeMoveBtn'),
      moveSummary: document.getElementById('moveSummary'),
      moveTargetSelect: document.getElementById('moveTargetSelect'),
      confirmMoveBtn: document.getElementById('confirmMoveBtn'),
      moveStatus: document.getElementById('moveStatus'),
      detailsModal: document.getElementById('detailsModal'),
      closeDetailsBtn: document.getElementById('closeDetailsBtn'),
      detailsFileName: document.getElementById('detailsFileName'),
      detailsFileNameInput: document.getElementById('detailsFileNameInput'),
      saveDetailsFileNameBtn: document.getElementById('saveDetailsFileNameBtn'),
      detailsFileID: document.getElementById('detailsFileID'),
      copyDetailsFileIDBtn: document.getElementById('copyDetailsFileIDBtn'),
      detailsBody: document.getElementById('detailsBody'),
      detailsStatus: document.getElementById('detailsStatus'),
      securityModal: document.getElementById('securityModal'),
      closeSecurityBtn: document.getElementById('closeSecurityBtn'),
      mfaRecoveryRemainingInput: document.getElementById('mfaRecoveryRemainingInput'),
      startTotpEnrollBtn: document.getElementById('startTotpEnrollBtn'),
      confirmTotpEnrollBtn: document.getElementById('confirmTotpEnrollBtn'),
      mfaPasskeyNameInput: document.getElementById('mfaPasskeyNameInput'),
      registerWebauthnBtn: document.getElementById('registerWebauthnBtn'),
      mfaPasskeysBody: document.getElementById('mfaPasskeysBody'),
      disableTotpBtn: document.getElementById('disableTotpBtn'),
      regenerateRecoveryBtn: document.getElementById('regenerateRecoveryBtn'),
      mfaLocalPasswordInput: document.getElementById('mfaLocalPasswordInput'),
      mfaLocalPasswordConfirmInput: document.getElementById('mfaLocalPasswordConfirmInput'),
      setLocalPasswordBtn: document.getElementById('setLocalPasswordBtn'),
      disableLocalPasswordBtn: document.getElementById('disableLocalPasswordBtn'),
      mfaTotpEnrollBox: document.getElementById('mfaTotpEnrollBox'),
      mfaTotpSecretInput: document.getElementById('mfaTotpSecretInput'),
      mfaTotpQR: document.getElementById('mfaTotpQR'),
      mfaTotpCodeInput: document.getElementById('mfaTotpCodeInput'),
      mfaRecoveryCodesBox: document.getElementById('mfaRecoveryCodesBox'),
      mfaRecoveryCodesOutput: document.getElementById('mfaRecoveryCodesOutput'),
      mfaStatus: document.getElementById('mfaStatus'),
      exportRecoveryBtn: document.getElementById('exportRecoveryBtn'),
      importRecoveryBtn: document.getElementById('importRecoveryBtn'),
      recoveryFileInput: document.getElementById('recoveryFileInput'),
      recoveryStatus: document.getElementById('recoveryStatus'),
      adminModal: document.getElementById('adminModal'),
      closeAdminBtn: document.getElementById('closeAdminBtn'),
      adminPartSizeInput: document.getElementById('adminPartSizeInput'),
      adminDocumentLimitInput: document.getElementById('adminDocumentLimitInput'),
      adminSafetyMarginInput: document.getElementById('adminSafetyMarginInput'),
      adminParallelInput: document.getElementById('adminParallelInput'),
      adminRateInput: document.getElementById('adminRateInput'),
      adminCooldownInput: document.getElementById('adminCooldownInput'),
      adminPublicPasswordMinInput: document.getElementById('adminPublicPasswordMinInput'),
      saveAdminUploadSettingsBtn: document.getElementById('saveAdminUploadSettingsBtn'),
      adminStatus: document.getElementById('adminStatus'),
      adminInstanceIDInput: document.getElementById('adminInstanceIDInput'),
      copyAdminInstanceIDBtn: document.getElementById('copyAdminInstanceIDBtn'),
      adminLicenseStatusInput: document.getElementById('adminLicenseStatusInput'),
      adminLicenseTierInput: document.getElementById('adminLicenseTierInput'),
      adminLicenseLimitsInput: document.getElementById('adminLicenseLimitsInput'),
      adminLicenseExpiresInput: document.getElementById('adminLicenseExpiresInput'),
      adminLicenseInstanceMatchInput: document.getElementById('adminLicenseInstanceMatchInput'),
      adminLicenseSummaryBadge: document.getElementById('adminLicenseSummaryBadge'),
      adminLicenseRawInput: document.getElementById('adminLicenseRawInput'),
      adminLicenseFileInput: document.getElementById('adminLicenseFileInput'),
      pickAdminLicenseFileBtn: document.getElementById('pickAdminLicenseFileBtn'),
      saveAdminLicenseBtn: document.getElementById('saveAdminLicenseBtn'),
      removeAdminLicenseBtn: document.getElementById('removeAdminLicenseBtn'),
      adminLicenseStatus: document.getElementById('adminLicenseStatus'),
      readOnlyBanner: document.getElementById('readOnlyBanner'),
      reconnectTelegramBtn: document.getElementById('reconnectTelegramBtn'),
    };

    function csrfToken() {
      const found = document.cookie.split('; ').find((entry) => entry.startsWith('td_csrf='));
      return found ? decodeURIComponent(found.slice('td_csrf='.length)) : '';
    }

    function parseRetryAfterSeconds(value) {
      const raw = String(value || '').trim();
      if (!raw) return 0;
      const seconds = Number.parseInt(raw, 10);
      if (Number.isFinite(seconds) && seconds > 0) return seconds;
      const absolute = Date.parse(raw);
      if (!Number.isFinite(absolute)) return 0;
      const diff = Math.ceil((absolute - Date.now()) / 1000);
      return diff > 0 ? diff : 0;
    }

    async function api(path, options = {}) {
      const headers = new Headers(options.headers || {});
      if (options.body && !(options.body instanceof Blob) && !headers.has('Content-Type')) {
        headers.set('Content-Type', 'application/json');
      }
      if (options.csrf) headers.set('X-CSRF-Token', csrfToken());
      const response = await fetch(path, {
        ...options,
        headers,
        credentials: 'same-origin',
      });
      if (!response.ok) {
        const retryAfterSeconds = parseRetryAfterSeconds(response.headers.get('Retry-After'));
        let code = '';
        let message = response.statusText;
        let payload = null;
        try {
          const body = await response.json();
          payload = body;
          code = String(body.error || '').trim();
          message = code || message;
        } catch (_) {}
        const err = new Error(humanizeError(message));
        err.code = code || String(message || '').trim();
        err.payload = payload;
        err.retryAfterSeconds = retryAfterSeconds;
        throw err;
      }
      if (response.status === 204) return null;
      return response.json();
    }

    function humanizeError(message) {
      const key = String(message || '').trim();
      if (!key) return 'Request failed.';
      const mapped = {
        share_list_failed: 'Failed to load shares.',
        share_recipients_list_failed: 'Failed to load share recipients.',
        telegram_session_not_found: 'Telegram session not found. Sign in again.',
        share_create_failed: 'Failed to create share.',
        share_revoke_failed: 'Failed to revoke share.',
        invalid_share_permission: 'Share permission must be read or read_delete.',
        file_not_found: 'File not found or access is no longer active.',
        file_delete_forbidden: 'You do not have delete access for this shared item.',
        public_link_list_failed: 'Failed to load public links.',
        public_link_create_failed: 'Failed to create public link.',
        public_link_revoke_failed: 'Failed to revoke public link.',
        download_failed: 'Failed to download file.',
        telegram_id_required: 'Telegram ID is required.',
        share_recipient_required: 'Select a recipient or enter Telegram ID.',
        invalid_max_downloads: 'Download limit must be a positive integer.',
        invalid_download_limit_mode: 'Download limit mode must be hard or soft.',
        invalid_public_link_password: 'Public link password does not match current policy.',
        network_upload_failed: 'Network error while uploading part. Check proxy/body limits and retry.',
        upload_part_network_failed: 'Network error while uploading part. Check proxy/body limits and retry.',
        upload_part_canceled: 'Upload part request was canceled.',
        upload_part_timeout: 'Upload part request timed out.',
        upload_part_http_failed: 'Upload part request was rejected by the server or proxy.',
        upload_resume_file_required: 'Select the same local file to resume this upload.',
        telegram_mfa_required: 'Telegram two-step password required.',
        telegram_mfa_invalid: 'Incorrect Telegram two-step password.',
        telegram_code_invalid: 'Incorrect Telegram login code.',
        telegram_code_expired: 'Telegram login code expired. Send a new code.',
        telegram_send_code_failed: 'Failed to request Telegram login code.',
        telegram_resend_code_failed: 'Failed to request next Telegram login code.',
        auth_rate_limited: 'Too many login code requests. Wait before requesting another code.',
        phone_invalid_format: 'Phone number format is invalid.',
        invalid_auth_challenge: 'Login challenge expired. Send a new code.',
        qr_login_not_found: 'QR login expired. Start QR login again.',
        recovery_replace_confirmation_required: 'Confirm recovery replace before importing.',
        recovery_snapshot_is_older: 'Recovery map is older than the latest snapshot for this account on this instance.',
        community_user_limit_reached: 'This instance allows only one connected Telegram account in Community version.',
        account_limit_reached: 'Connected account limit reached for current edition.',
        workspace_limit_reached: 'Workspace limit reached for current edition.',
        license_required: 'License JSON is required.',
        license_invalid: 'License payload is invalid.',
        license_verify_failed: 'License signature verification failed.',
        license_install_failed: 'Failed to install license.',
        license_state_load_failed: 'Failed to load license state.',
        license_keys_not_configured: 'License public keys are not configured.',
        invite_required: 'Access requires an invite from the instance owner.',
        local_mfa_required: 'Local instance 2FA required.',
        local_mfa_code_required: 'Enter local 2FA code.',
        local_mfa_code_invalid: 'Local 2FA code is invalid.',
        local_mfa_not_configured: 'Local 2FA is not configured yet.',
        local_mfa_state_failed: 'Failed to load local 2FA state.',
        local_mfa_enroll_start_failed: 'Failed to start local 2FA enrollment.',
        local_mfa_enroll_confirm_failed: 'Failed to confirm local 2FA enrollment.',
        local_mfa_verify_failed: 'Failed to verify local 2FA code.',
        local_password_invalid: 'Local password is invalid.',
        local_password_not_configured: 'Local password is not configured.',
        local_password_set_failed: 'Failed to set local password.',
        local_password_verify_failed: 'Failed to verify local password.',
        local_password_disable_failed: 'Failed to disable local password.',
        local_password_mismatch: 'Passwords do not match.',
        local_password_too_short: 'Local password must be at least 5 characters.',
        local_mfa_disable_failed: 'Failed to disable local TOTP.',
        webauthn_disable_failed: 'Failed to disable passkeys.',
        webauthn_rename_failed: 'Failed to rename passkey.',
        webauthn_credential_required: 'Passkey identifier is required.',
        webauthn_credential_not_found: 'Passkey was not found.',
        recovery_codes_disable_failed: 'Failed to disable recovery codes.',
        recovery_codes_required_for_totp: 'Recovery codes are required while TOTP is enabled.',
        remember_device_invalid: 'Saved device login expired. Sign in with Telegram again.',
        remember_device_lookup_failed: 'Failed to load saved device login.',
        remember_device_persist_failed: 'Failed to save this device.',
        telegram_login_required: 'Telegram login is required for this account.',
        telegram_session_missing_read_only: 'Telegram session is missing. Instance is in read-only map mode.',
        telegram_session_stale: 'Telegram session is no longer valid. Reconnect Telegram for write access.',
        telegram_session_check_failed: 'Unable to verify Telegram session right now. Try again in a moment.',
        telegram_account_mismatch: 'This Telegram account does not match the current TeleVault account.',
        telegram_session_status_failed: 'Failed to check Telegram session status.',
        recovery_code_required: 'Enter recovery code.',
        recovery_code_invalid: 'Recovery code is invalid.',
        recovery_code_verify_failed: 'Failed to verify recovery code.',
        recovery_codes_regenerate_failed: 'Failed to regenerate recovery codes.',
        webauthn_not_configured: 'WebAuthn is not configured for this account.',
        webauthn_challenge_required: 'WebAuthn challenge is required.',
        webauthn_challenge_invalid: 'WebAuthn challenge is invalid or expired.',
        webauthn_configuration_invalid: 'WebAuthn configuration is invalid for this host.',
        webauthn_registration_start_failed: 'Failed to start WebAuthn registration.',
        webauthn_registration_finish_failed: 'Failed to finish WebAuthn registration.',
        webauthn_verify_start_failed: 'Failed to start WebAuthn verification.',
        webauthn_verify_failed: 'WebAuthn verification failed.',
      };
      return mapped[key] || key;
    }

    const PHONE_COUNTRIES = [
      { id: 'ua', iso: 'UA', dial: '380', mask: '00-000-00-00' },
      { id: 'pl', iso: 'PL', dial: '48', mask: '000-000-000' },
      { id: 'de', iso: 'DE', dial: '49', mask: '0000-0000000' },
      { id: 'fr', iso: 'FR', dial: '33', mask: '0-00-00-00-00' },
      { id: 'es', iso: 'ES', dial: '34', mask: '000-000-000' },
      { id: 'it', iso: 'IT', dial: '39', mask: '000-000-0000' },
      { id: 'cz', iso: 'CZ', dial: '420', mask: '000-000-000' },
      { id: 'lt', iso: 'LT', dial: '370', mask: '000-00000' },
      { id: 'lv', iso: 'LV', dial: '371', mask: '00-000-000' },
      { id: 'ee', iso: 'EE', dial: '372', mask: '0000-0000' },
      { id: 'nl', iso: 'NL', dial: '31', mask: '00-000-0000' },
      { id: 'gb', iso: 'GB', dial: '44', mask: '0000-000-000' },
      { id: 'usca', iso: 'US', dial: '1', mask: '000-000-0000' },
      { id: 'tr', iso: 'TR', dial: '90', mask: '000-000-0000' },
      { id: 'md', iso: 'MD', dial: '373', mask: '000-00-000' },
      { id: 'ro', iso: 'RO', dial: '40', mask: '000-000-000' },
      { id: 'ge', iso: 'GE', dial: '995', mask: '000-000-000' },
      { id: 'int', iso: 'INT', dial: '', mask: '+0 00-000-00-00' },
    ];

    const TELEGRAM_RETRY_TYPES = {
      sms: 'SMS',
      call: 'Call',
      flash_call: 'Flash call',
      missed_call: 'Missed call',
      fragment_sms: 'Fragment SMS',
    };

    function normalizeTelegramRetryType(value) {
      return String(value || '').trim();
    }

    function supportedTelegramRetryType(value) {
      const normalized = normalizeTelegramRetryType(value);
      if (!normalized) return '';
      return Object.prototype.hasOwnProperty.call(TELEGRAM_RETRY_TYPES, normalized) ? normalized : '';
    }

    function telegramRetryLabel(value) {
      const normalized = supportedTelegramRetryType(value);
      return normalized ? TELEGRAM_RETRY_TYPES[normalized] : '';
    }

    function phoneCountryByID(id) {
      return PHONE_COUNTRIES.find((entry) => entry.id === id) || PHONE_COUNTRIES[0];
    }

    function detectPhoneCountryByDial(allDigits) {
      const candidates = PHONE_COUNTRIES.filter((entry) => entry.dial);
      candidates.sort((left, right) => right.dial.length - left.dial.length);
      return candidates.find((entry) => allDigits.startsWith(entry.dial)) || null;
    }

    function normalizePhone(rawValue, countryID) {
      const raw = String(rawValue || '').trim();
      if (!raw) return { ok: false, reason: 'empty' };
      const hasPlus = raw.startsWith('+');
      const digits = raw.replace(/\D+/g, '');
      if (!digits) return { ok: false, reason: 'empty' };

      if (hasPlus) {
        if (digits.length < 8 || digits.length > 15) return { ok: false, reason: 'length' };
        const detected = detectPhoneCountryByDial(digits);
        return {
          ok: true,
          e164: `+${digits}`,
          countryID: detected ? detected.id : 'int',
        };
      }

      const detectedNoPlus = detectPhoneCountryByDial(digits);
      if (detectedNoPlus && digits.length >= 8 && digits.length <= 15 && digits.length > detectedNoPlus.dial.length+4) {
        return {
          ok: true,
          e164: `+${digits}`,
          countryID: detectedNoPlus.id,
        };
      }

      const country = phoneCountryByID(countryID);
      if (!country || !country.dial) return { ok: false, reason: 'intl_required' };
      let localDigits = digits.replace(/^0+/, '');
      if (!localDigits) return { ok: false, reason: 'empty' };
      const fullDigits = `${country.dial}${localDigits}`;
      if (fullDigits.length < 8 || fullDigits.length > 15) return { ok: false, reason: 'length' };
      return {
        ok: true,
        e164: `+${fullDigits}`,
        countryID: country.id,
      };
    }

    function refreshPhonePreview() {
      const normalized = normalizePhone(el.loginPhoneInput.value, state.phoneCountry);
      if (!normalized.ok) {
        state.normalizedPhone = '';
        if (!String(el.loginPhoneInput.value || '').trim()) {
          el.loginPhonePreview.textContent = '';
        } else if (normalized.reason === 'intl_required') {
          el.loginPhonePreview.textContent = 'Use international format: +<country code><number>.';
        } else {
          el.loginPhonePreview.textContent = 'Phone format looks invalid.';
        }
        return;
      }
      if (normalized.countryID !== state.phoneCountry && normalized.countryID !== 'int') {
        state.phoneCountry = normalized.countryID;
        el.loginCountrySelect.value = normalized.countryID;
        updatePhoneCountryHint();
      }
      state.normalizedPhone = normalized.e164;
      el.loginPhonePreview.textContent = `Will send code to ${normalized.e164}`;
    }

    function updatePhoneCountryHint() {
      const country = phoneCountryByID(state.phoneCountry);
      if (!country.dial) {
        el.loginPhoneInput.placeholder = country.mask;
        el.loginPhoneHint.textContent = `Manual format: ${country.mask}`;
        return;
      }
      el.loginPhoneInput.placeholder = country.mask;
      el.loginPhoneHint.textContent = `Format: ${country.mask} (or paste full international number).`;
    }

    function setPhoneCountryOptions() {
      const rows = PHONE_COUNTRIES.map((entry) => {
        const dial = entry.dial ? `+${entry.dial}` : 'manual';
        return `<option value="${entry.id}">${entry.iso} ${dial}</option>`;
      });
      el.loginCountrySelect.innerHTML = rows.join('');
      if (!PHONE_COUNTRIES.some((entry) => entry.id === state.phoneCountry)) {
        state.phoneCountry = PHONE_COUNTRIES[0].id;
      }
      el.loginCountrySelect.value = state.phoneCountry;
      updatePhoneCountryHint();
      refreshPhonePreview();
    }

    function requireNormalizedPhone() {
      refreshPhonePreview();
      if (!state.normalizedPhone) {
        throw new Error(humanizeError('phone_invalid_format'));
      }
      return state.normalizedPhone;
    }

    function clearTelegramCodeRetryState() {
      clearInterval(state.telegramCodeRetryTimer);
      state.telegramCodeRetryTimer = null;
      state.telegramCodeRetryAt = 0;
      state.telegramCodeNextType = '';
      state.telegramCodeCanResend = false;
      state.telegramCodePhone = '';
      el.sendCodeBtn.textContent = 'Send code';
      el.sendCodeBtn.disabled = false;
    }

    function updateTelegramCodeRetryButton() {
      const now = Date.now();
      const retryAt = Number(state.telegramCodeRetryAt || 0);
      if (retryAt > now) {
        const remaining = Math.max(1, Math.ceil((retryAt - now) / 1000));
        el.sendCodeBtn.disabled = true;
        el.sendCodeBtn.textContent = `Retry in ${remaining}s`;
        return;
      }

      const retryType = supportedTelegramRetryType(state.telegramCodeNextType);
      if (state.telegramCodeCanResend && retryType) {
        el.sendCodeBtn.disabled = false;
        el.sendCodeBtn.textContent = `Try ${telegramRetryLabel(retryType)}`;
        return;
      }

      el.sendCodeBtn.disabled = false;
      el.sendCodeBtn.textContent = 'Send code';
    }

    function applyTelegramCodeDeliveryState(delivery, phone) {
      const payload = delivery && typeof delivery === 'object' ? delivery : {};
      const timeout = Number(payload.timeout_seconds || 0);
      const nextType = normalizeTelegramRetryType(payload.next_type);
      clearInterval(state.telegramCodeRetryTimer);
      state.telegramCodeRetryTimer = null;
      state.telegramCodeRetryAt = 0;
      state.telegramCodePhone = phone;
      state.telegramCodeNextType = nextType;
      state.telegramCodeCanResend = Boolean(supportedTelegramRetryType(nextType));
      if (timeout > 0) {
        state.telegramCodeRetryAt = Date.now() + (timeout * 1000);
        state.telegramCodeRetryTimer = setInterval(() => {
          if (Date.now() >= state.telegramCodeRetryAt) {
            clearInterval(state.telegramCodeRetryTimer);
            state.telegramCodeRetryTimer = null;
            state.telegramCodeRetryAt = 0;
          }
          updateTelegramCodeRetryButton();
        }, 1000);
      }
      updateTelegramCodeRetryButton();
    }

    function applyTelegramRateLimitCooldown(retryAfterSeconds) {
      const cooldown = Number(retryAfterSeconds || 0);
      if (cooldown <= 0) return;
      clearInterval(state.telegramCodeRetryTimer);
      state.telegramCodeRetryTimer = null;
      state.telegramCodeRetryAt = Date.now() + (cooldown * 1000);
      state.telegramCodeRetryTimer = setInterval(() => {
        if (Date.now() >= state.telegramCodeRetryAt) {
          clearInterval(state.telegramCodeRetryTimer);
          state.telegramCodeRetryTimer = null;
          state.telegramCodeRetryAt = 0;
        }
        updateTelegramCodeRetryButton();
      }, 1000);
      updateTelegramCodeRetryButton();
    }

    function describeTelegramCodeDelivery(delivery) {
      const payload = delivery && typeof delivery === 'object' ? delivery : {};
      const type = String(payload.type || '').trim();
      const length = Number(payload.length || 0);
      const timeout = Number(payload.timeout_seconds || 0);
      const nextType = String(payload.next_type || '').trim();
      const details = [];

      switch (type) {
        case 'app':
          details.push('Code sent to Telegram app.');
          break;
        case 'sms':
          details.push('Code sent by SMS.');
          break;
        case 'call':
          details.push('Code will be delivered by phone call.');
          break;
        case 'flash_call':
          details.push('Code will be delivered via flash call.');
          break;
        case 'missed_call':
          details.push('Code will be delivered via missed call.');
          break;
        case 'fragment_sms':
          details.push('Code delivered via Fragment.');
          break;
        case 'firebase_sms':
          details.push('Code will be delivered via Firebase SMS.');
          break;
        case 'sms_word':
          details.push('Code sent by SMS as a word.');
          break;
        case 'sms_phrase':
          details.push('Code sent by SMS as a phrase.');
          break;
        case 'email':
          details.push('Code sent by email.');
          break;
        case 'email_setup_required':
          details.push('Email login setup is required in Telegram.');
          break;
        default:
          details.push('Code sent.');
          break;
      }

      if (length > 0) {
        details.push(`Code length: ${length}.`);
      }
      if (timeout > 0) {
        details.push(`Wait ${timeout}s before retry.`);
      }
      if (nextType) {
        details.push(`Next method: ${nextType}.`);
      }
      details.push('Enter the code and sign in.');
      return details.join(' ');
    }

    function uploadHTTPErrorMessage(status, statusText, code) {
      const normalizedStatus = Number(status) || 0;
      const suffix = normalizedStatus > 0 ? ` (${normalizedStatus}${statusText ? ` ${statusText}` : ''})` : '';
      if (code && code !== 'upload_part_http_failed') return `${humanizeError(code)}${suffix}`;
      return `${humanizeError('upload_part_http_failed')}${suffix}`;
    }

    function getPublicLinkPasswordMinLength() {
      const raw = Number(state.appInfo && state.appInfo.public_link_password_min_length);
      if (!Number.isFinite(raw)) return 8;
      const normalized = Math.floor(raw);
      if (normalized < 1) return 1;
      if (normalized > 1024) return 1024;
      return normalized;
    }

    function applyPublicLinkPasswordPolicy() {
      const minLength = getPublicLinkPasswordMinLength();
      el.publicPasswordInput.minLength = String(minLength);
      el.publicPasswordInput.placeholder = `Optional, at least ${minLength} characters`;
    }

    function applyReadOnlyMode() {
      const readOnly = Boolean(state.readOnlyMapMode);
      el.readOnlyBanner.classList.toggle('hidden', !readOnly);
      el.createFolderBtn.disabled = readOnly;
      el.uploadBtn.disabled = readOnly;
      el.deleteSelectedBtn.disabled = readOnly;
      el.moveSelectedBtn.disabled = readOnly;
      el.dropZone.classList.toggle('disabled', readOnly);
    }

    function readOnlyMapModeMessage() {
      if (state.telegramSessionStatus === 'stale') {
        return humanizeError('telegram_session_stale');
      }
      return humanizeError('telegram_session_missing_read_only');
    }

    function requireWritableAction(setStatus) {
      if (!state.readOnlyMapMode) {
        return true;
      }
      if (typeof setStatus === 'function') {
        setStatus(readOnlyMapModeMessage(), true);
      } else {
        setUploadStatus(readOnlyMapModeMessage(), true);
      }
      return false;
    }

    function showApp(user, session) {
      state.user = user;
      state.readOnlyMapMode = Boolean(session && session.read_only_map_mode);
      state.telegramSessionStatus = session && session.telegram_session_status ? String(session.telegram_session_status) : '';
      state.reconnectMode = false;
      el.loginView.classList.add('hidden');
      el.appView.classList.remove('hidden');
      el.userbar.classList.remove('hidden');
      el.userName.textContent = user.displayName || user.username || `Telegram ${user.telegram_id}`;
      el.adminBtn.classList.toggle('hidden', user.role !== 'admin');
      applyReadOnlyMode();
      restoreFolderState();
      restoreUploadQueueState();
      if (!window.location.hash) window.location.hash = state.currentFolderId ? folderRoute(state.currentFolderId) : '#/';
      applyRoute();
      monitorRestoredUploads();
    }

    function showLogin(message) {
      state.user = null;
      state.currentFolderId = '';
      state.folderStack = [];
      state.view = 'own';
      state.droppedFiles = [];
      state.selectedFileIds = new Set();
      state.draggingItems = [];
      state.shareFile = null;
      state.detailsFile = null;
      state.detailsDownloadActivity = null;
      state.uploadQueue = [];
      state.uploadQueueRunning = false;
      state.uploadMonitorRunning = false;
      state.nextUploadQueueID = 1;
      state.mfaStatus = null;
      state.readOnlyMapMode = false;
      state.telegramSessionStatus = '';
      state.localMFAMethods = [];
      state.reconnectMode = false;
      persistUploadQueueState();
      renderUploadQueue();
      closeDetailsDialog();
      closeSecurityDialog();
      el.loginView.classList.remove('hidden');
      el.appView.classList.add('hidden');
      el.userbar.classList.add('hidden');
      el.adminBtn.classList.add('hidden');
      resetLoginForm();
      el.loginStatus.textContent = message || 'Not signed in.';
      updateTabSafetyIndicator();
      applyReadOnlyMode();
    }

    function setTelegramCodeInputDefaults() {
      el.loginCodeLabel.textContent = 'Code';
      el.loginCodeInput.name = 'telegram_code';
      el.loginCodeInput.autocomplete = 'one-time-code';
      el.loginCodeInput.inputMode = 'numeric';
      el.loginCodeInput.pattern = '[0-9]*';
      el.loginCodeInput.autocapitalize = 'none';
      el.loginCodeInput.spellcheck = false;
      el.loginCodeInput.placeholder = '12345';
    }

    function setLocalUnifiedCodeInputDefaults() {
      el.loginCodeLabel.textContent = '2FA code or recovery key';
      el.loginCodeInput.name = 'totp';
      el.loginCodeInput.autocomplete = 'one-time-code';
      el.loginCodeInput.inputMode = 'text';
      el.loginCodeInput.pattern = '';
      el.loginCodeInput.autocapitalize = 'none';
      el.loginCodeInput.spellcheck = false;
      el.loginCodeInput.placeholder = '123456 or ABCD-EFGH-IJKL';
    }

    function resetLocalPasswordField() {
      el.localPasswordInput.type = 'password';
      el.localPasswordInput.value = '';
      el.localPasswordToggleBtn.classList.remove('visible');
      el.localPasswordToggleBtn.setAttribute('aria-label', 'Show local password');
      el.localPasswordToggleBtn.setAttribute('title', 'Show local password');
    }

    function resetLoginForm() {
      clearInterval(state.qrTimer);
      clearInterval(state.qrCountdownTimer);
      clearTelegramCodeRetryState();
      state.qrLoginId = null;
      state.qrExpiresAt = null;
      state.loginMFAContext = '';
      state.loginAltAccountMode = false;
      state.reconnectMode = false;
      el.loginStatus.classList.remove('error');
      el.loginPhoneRow.classList.remove('hidden');
      el.loginCountryField.classList.remove('hidden');
      el.loginPhoneField.classList.remove('hidden');
      el.localPasswordAction.classList.add('hidden');
      el.localPasswordField.classList.add('hidden');
      el.loginUseWebauthnBtn.classList.add('hidden');
      resetLocalPasswordField();
      el.qrImage.classList.add('hidden');
      el.qrImage.src = '';
      el.loginCodeField.classList.add('hidden');
      setTelegramCodeInputDefaults();
      el.loginPasswordField.classList.add('hidden');
      el.loginPasswordLabel.textContent = 'Telegram two-step password (if required)';
      el.loginPasswordInput.placeholder = 'Telegram two-step password';
      el.loginWithCodeBtn.classList.add('hidden');
      el.loginCodeInput.value = '';
      el.loginPasswordInput.value = '';
      refreshPhonePreview();
      applyLoginEntryMode();
    }

    function applyLoginEntryMode() {
      const hasRemembered = Boolean(state.rememberedUser);
      const showTelegramLogin = !hasRemembered || state.loginAltAccountMode;
      el.loginTelegramBox.classList.toggle('hidden', !showTelegramLogin);
      el.startQrBtn.classList.toggle('hidden', !showTelegramLogin || state.reconnectMode);
      el.sendCodeBtn.classList.toggle('hidden', !showTelegramLogin);
      el.useAnotherAccountBtn.classList.toggle('hidden', !hasRemembered);

      if (!showTelegramLogin) {
        el.loginCodeField.classList.add('hidden');
        el.loginPasswordField.classList.add('hidden');
        el.loginWithCodeBtn.classList.add('hidden');
        el.localPasswordAction.classList.add('hidden');
        el.localPasswordField.classList.add('hidden');
        el.loginUseWebauthnBtn.classList.add('hidden');
        el.qrImage.classList.add('hidden');
        el.qrImage.src = '';
        clearInterval(state.qrTimer);
        clearInterval(state.qrCountdownTimer);
        clearTelegramCodeRetryState();
      }
    }

    async function loadMe() {
      try {
        await loadAppInfo();
        const data = await api('/me');
        showApp(data.user, data.session || null);
      } catch (_) {
        showLogin();
        await loadRememberedAccount();
      }
    }

    async function loadAppInfo() {
      try {
        const info = await api('/app-info');
        state.appInfo = info;
        applyPublicLinkPasswordPolicy();
        const version = info && info.build && info.build.version ? info.build.version : '';
        el.appVersion.textContent = version ? `v${version}` : '';
        state.uploadDebugAllowed = true;
        const storedDebug = localStorage.getItem('tdv.uploadDebug') === '1';
        const urlDebug = new URLSearchParams(window.location.search).get('upload_debug') === '1';
        setUploadDebugEnabled(Boolean(info && info.debug) || storedDebug || urlDebug);
      } catch (_) {
        state.appInfo = null;
        applyPublicLinkPasswordPolicy();
        state.uploadDebugAllowed = true;
        setUploadDebugEnabled(localStorage.getItem('tdv.uploadDebug') === '1');
      }
    }

    async function loadRememberedAccount() {
      el.rememberedBox.classList.add('hidden');
      state.rememberedUser = null;
      state.loginAltAccountMode = false;
      try {
        const data = await api('/auth/remembered-account');
        if (!data || !data.available || !data.user) {
          applyLoginEntryMode();
          return;
        }
        state.rememberedUser = data.user;
        state.localMFAMethods = Array.isArray(data.methods) ? data.methods.slice() : [];
        el.rememberedUser.textContent = `Continue as ${data.user.displayName || data.user.username || `Telegram ${data.user.telegram_id}`}`;
        const tg = String(data.telegram_session_status || '').trim();
        el.rememberedStatus.textContent = tg === 'ok'
          ? 'Telegram session linked.'
          : 'Telegram session missing: map-only mode until re-link.';
        el.rememberedBox.classList.remove('hidden');
      } catch (_) {
      }
      applyLoginEntryMode();
    }

    async function continueRememberedLogin() {
      el.continueRememberedBtn.disabled = true;
      el.loginStatus.classList.remove('error');
      el.loginStatus.textContent = 'Signing in...';
      try {
        await api('/auth/remembered-login', { method: 'POST' });
      } catch (err) {
        if (err && err.code === 'local_mfa_required') {
          try {
            await startLocalMFAFlow(err);
            return;
          } catch (mfaErr) {
            el.loginStatus.classList.add('error');
            el.loginStatus.textContent = mfaErr.message;
            return;
          }
        }
        el.loginStatus.classList.add('error');
        el.loginStatus.textContent = err.message;
        await loadRememberedAccount();
        return;
      } finally {
        el.continueRememberedBtn.disabled = false;
      }
      await loadMe();
    }

    async function forgetRememberedDevice() {
      el.forgetRememberedBtn.disabled = true;
      try {
        await api('/auth/remembered-device/forget', { method: 'POST', csrf: true });
      } catch (_) {
      } finally {
        el.forgetRememberedBtn.disabled = false;
      }
      state.rememberedUser = null;
      el.rememberedBox.classList.add('hidden');
      el.loginStatus.classList.remove('error');
      el.loginStatus.textContent = 'Saved local sign-in removed. Use Telegram sign-in again.';
      state.loginAltAccountMode = true;
      applyLoginEntryMode();
    }

    function useAnotherAccount() {
      clearInterval(state.qrTimer);
      clearInterval(state.qrCountdownTimer);
      clearTelegramCodeRetryState();
      state.qrLoginId = null;
      state.qrExpiresAt = null;
      state.loginMFAContext = '';
      state.reconnectMode = false;
      state.loginAltAccountMode = true;
      el.loginCodeField.classList.add('hidden');
      el.loginPasswordField.classList.add('hidden');
      el.loginWithCodeBtn.classList.add('hidden');
      el.loginCodeInput.value = '';
      el.loginPasswordInput.value = '';
      el.qrImage.classList.add('hidden');
      el.qrImage.src = '';
      el.loginPhoneRow.classList.remove('hidden');
      applyLoginEntryMode();
      el.loginStatus.classList.remove('error');
      el.loginStatus.textContent = 'Signing in with another Telegram account.';
      el.loginPhoneInput.focus();
    }

    function startReconnectTelegram() {
      showLogin('Reconnect Telegram for this account.');
      state.loginAltAccountMode = true;
      state.reconnectMode = true;
      applyLoginEntryMode();
      el.loginStatus.classList.remove('error');
      el.loginStatus.textContent = 'Sign in with the same Telegram account to reconnect this session.';
      el.loginPhoneInput.focus();
    }

    async function startQR() {
      clearInterval(state.qrTimer);
      clearInterval(state.qrCountdownTimer);
      clearTelegramCodeRetryState();
      state.qrLoginId = null;
      state.qrExpiresAt = null;
      state.loginMFAContext = '';
      el.loginCodeField.classList.add('hidden');
      el.loginPasswordField.classList.add('hidden');
      el.loginWithCodeBtn.classList.add('hidden');
      el.loginPasswordInput.value = '';
      el.startQrBtn.disabled = true;
      el.loginStatus.classList.remove('error');
      el.loginStatus.textContent = 'Starting...';
      try {
        const data = await api('/auth/telegram/qr/start', { method: 'POST' });
        updateQR(data.qr_login);
        state.qrTimer = setInterval(pollQR, 2000);
      } catch (err) {
        el.loginStatus.classList.add('error');
        el.loginStatus.textContent = err.message;
      } finally {
        el.startQrBtn.disabled = false;
      }
    }

    async function sendTelegramCode() {
      clearInterval(state.qrTimer);
      clearInterval(state.qrCountdownTimer);
      state.qrLoginId = null;
      state.qrExpiresAt = null;
      el.qrImage.classList.add('hidden');
      state.loginMFAContext = '';
      el.loginPasswordField.classList.add('hidden');
      el.loginPasswordInput.value = '';
      let phone = '';
      try {
        phone = requireNormalizedPhone();
      } catch (err) {
        el.loginStatus.classList.add('error');
        el.loginStatus.textContent = err.message;
        return;
      }
      const retryType = supportedTelegramRetryType(state.telegramCodeNextType);
      const canResend = Boolean(state.telegramCodeCanResend &&
        retryType &&
        Number(state.telegramCodeRetryAt || 0) <= Date.now() &&
        state.telegramCodePhone === phone);
      const endpoint = canResend ? '/auth/telegram/resend-code' : '/auth/telegram/send-code';
      el.sendCodeBtn.disabled = true;
      el.loginStatus.classList.remove('error');
      el.loginStatus.textContent = canResend ? `Requesting ${telegramRetryLabel(retryType)} code...` : 'Sending code...';
      try {
        const data = await api(endpoint, {
          method: 'POST',
          body: JSON.stringify({ phone }),
        });
        state.loginMFAContext = 'phone';
        el.loginCodeField.classList.remove('hidden');
        el.loginWithCodeBtn.classList.remove('hidden');
        applyTelegramCodeDeliveryState(data && data.delivery, phone);
        el.loginStatus.textContent = describeTelegramCodeDelivery(data && data.delivery);
        el.loginCodeInput.focus();
      } catch (err) {
        el.loginStatus.classList.add('error');
        if (err && err.code === 'invalid_auth_challenge') {
          clearTelegramCodeRetryState();
          el.loginCodeInput.value = '';
        }
        if (err && err.code === 'auth_rate_limited') {
          applyTelegramRateLimitCooldown(err.retryAfterSeconds);
          const retryAfterSeconds = Number(err.retryAfterSeconds || 0);
          el.loginStatus.textContent = retryAfterSeconds > 0
            ? `Too many login code requests. Try again in ${retryAfterSeconds}s.`
            : err.message;
        } else {
          el.loginStatus.textContent = err.message;
        }
      } finally {
        updateTelegramCodeRetryButton();
      }
    }

    async function loginWithCode() {
      const password = String(el.loginPasswordInput.value || '');
      const localPassword = String(el.localPasswordInput.value || '');
      const code = String(el.loginCodeInput.value || '').trim();
      el.loginWithCodeBtn.disabled = true;
      el.loginStatus.classList.remove('error');
      el.loginStatus.textContent = 'Signing in...';
      try {
        if (state.loginMFAContext === 'local_totp_setup') {
          if (!code) {
            el.loginStatus.classList.add('error');
            el.loginStatus.textContent = 'Enter local 2FA code.';
            el.loginCodeInput.focus();
            return;
          }
          const data = await api('/auth/mfa/totp/enroll/confirm', {
            method: 'POST',
            csrf: true,
            body: JSON.stringify({ code }),
          });
          if (Array.isArray(data.recovery_codes) && data.recovery_codes.length > 0) {
            el.loginStatus.textContent = `Local 2FA enabled. Save recovery codes now: ${data.recovery_codes.join(', ')}`;
          }
          showApp(data.user);
          return;
        }
        if (state.loginMFAContext === 'local_code_verify') {
          if (hasMethod('password') && !el.localPasswordField.classList.contains('hidden') && localPassword) {
            const data = await api('/auth/local-password/verify', {
              method: 'POST',
              csrf: true,
              body: JSON.stringify({ password: localPassword }),
            });
            showApp(data.user);
            return;
          }
          if (!code) {
            el.loginStatus.classList.add('error');
            el.loginStatus.textContent = 'Enter 2FA code or recovery key.';
            el.loginCodeInput.focus();
            return;
          }
          const localCodeResolution = resolveLocalCodeVerificationInput(code);
          if (!localCodeResolution.ok) {
            el.loginStatus.classList.add('error');
            el.loginStatus.textContent = localCodeResolution.message;
            el.loginCodeInput.focus();
            return;
          }
          const data = await api(localCodeResolution.path, {
            method: 'POST',
            csrf: true,
            body: JSON.stringify({ code: localCodeResolution.code }),
          });
          showApp(data.user);
          return;
        }
        if (state.loginMFAContext === 'local_password_verify') {
          if (!localPassword) {
            el.loginStatus.classList.add('error');
            el.loginStatus.textContent = 'Enter local password.';
            el.localPasswordInput.focus();
            return;
          }
          const data = await api('/auth/local-password/verify', {
            method: 'POST',
            csrf: true,
            body: JSON.stringify({ password: localPassword }),
          });
          showApp(data.user);
          return;
        }

        if (state.qrLoginId && state.loginMFAContext === 'qr') {
          if (!password) {
            el.loginStatus.classList.add('error');
            el.loginStatus.textContent = 'Telegram two-step password required.';
            el.loginPasswordInput.focus();
            return;
          }
          const qrPayload = { qr_login_id: state.qrLoginId, password };
          const data = await api('/auth/telegram/qr/complete', {
            method: 'POST',
            body: JSON.stringify(qrPayload),
          });
          clearInterval(state.qrTimer);
          clearInterval(state.qrCountdownTimer);
          state.qrLoginId = null;
          state.qrExpiresAt = null;
          el.qrImage.classList.add('hidden');
          showApp(data.user);
          return;
        }

        let phone = '';
        try {
          phone = requireNormalizedPhone();
        } catch (err) {
          el.loginStatus.classList.add('error');
          el.loginStatus.textContent = err.message;
          return;
        }
        if (!code) {
          el.loginStatus.classList.add('error');
          el.loginStatus.textContent = 'Code is required.';
          return;
        }
        const payload = { phone, code };
        if (password) {
          payload.password = password;
        }
        const authPath = state.reconnectMode ? '/auth/telegram/reconnect' : '/auth/telegram/login';
        const data = await api(authPath, {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        clearInterval(state.qrTimer);
        clearInterval(state.qrCountdownTimer);
        state.qrLoginId = null;
        state.qrExpiresAt = null;
        el.qrImage.classList.add('hidden');
        showApp(data.user, data.session || null);
      } catch (err) {
        if (err && err.code === 'telegram_mfa_required') {
          state.loginMFAContext = state.qrLoginId ? 'qr' : 'phone';
          showLoginMFA(true, state.loginMFAContext);
          el.loginStatus.textContent = 'Telegram two-step password required.';
          el.loginPasswordInput.focus();
        } else if (err && err.code === 'local_mfa_required') {
          try {
            await startLocalMFAFlow(err);
          } catch (mfaErr) {
            el.loginStatus.classList.add('error');
            el.loginStatus.textContent = mfaErr.message;
          }
        } else {
          el.loginStatus.classList.add('error');
          if (err && err.code === 'telegram_mfa_invalid') {
            showLoginMFA(true, state.loginMFAContext || (state.qrLoginId ? 'qr' : 'phone'));
            el.loginPasswordInput.value = '';
            el.loginPasswordInput.focus();
          }
          el.loginStatus.textContent = err.message;
        }
      } finally {
        el.loginWithCodeBtn.disabled = false;
      }
    }

    function decodeBase64URL(value) {
      if (!value) return new Uint8Array();
      const padded = value.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(value.length / 4) * 4, '=');
      const binary = atob(padded);
      const bytes = new Uint8Array(binary.length);
      for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i);
      return bytes;
    }

    function encodeBase64URL(bytes) {
      const input = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes || []);
      let binary = '';
      for (let i = 0; i < input.length; i += 1) binary += String.fromCharCode(input[i]);
      return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
    }

    function normalizeWebAuthnRequestOptions(raw) {
      const envelope = raw && raw.public_key ? raw.public_key : raw;
      const publicKey = envelope && envelope.publicKey ? envelope.publicKey : envelope;
      if (!publicKey) return null;
      const normalized = { ...publicKey };
      if (normalized.challenge) normalized.challenge = decodeBase64URL(normalized.challenge);
      if (Array.isArray(normalized.allowCredentials)) {
        normalized.allowCredentials = normalized.allowCredentials.map((item) => ({
          ...item,
          id: decodeBase64URL(item.id),
        }));
      }
      return normalized;
    }

    function normalizeWebAuthnCreationOptions(raw) {
      const envelope = raw && raw.public_key ? raw.public_key : raw;
      const publicKey = envelope && envelope.publicKey ? envelope.publicKey : envelope;
      if (!publicKey) return null;
      const normalized = { ...publicKey };
      if (normalized.challenge) normalized.challenge = decodeBase64URL(normalized.challenge);
      if (normalized.user && normalized.user.id) {
        normalized.user = { ...normalized.user, id: decodeBase64URL(normalized.user.id) };
      }
      if (Array.isArray(normalized.excludeCredentials)) {
        normalized.excludeCredentials = normalized.excludeCredentials.map((item) => ({
          ...item,
          id: decodeBase64URL(item.id),
        }));
      }
      return normalized;
    }

    function serializeWebAuthnCredential(credential) {
      if (!credential) return null;
      const response = credential.response || {};
      const out = {
        id: credential.id,
        rawId: encodeBase64URL(new Uint8Array(credential.rawId)),
        type: credential.type,
        response: {
          clientDataJSON: response.clientDataJSON ? encodeBase64URL(new Uint8Array(response.clientDataJSON)) : '',
        },
      };
      if (response.attestationObject) {
        out.response.attestationObject = encodeBase64URL(new Uint8Array(response.attestationObject));
      }
      if (response.authenticatorData) {
        out.response.authenticatorData = encodeBase64URL(new Uint8Array(response.authenticatorData));
      }
      if (response.signature) {
        out.response.signature = encodeBase64URL(new Uint8Array(response.signature));
      }
      if (response.userHandle) {
        out.response.userHandle = encodeBase64URL(new Uint8Array(response.userHandle));
      }
      if (credential.getClientExtensionResults) {
        out.clientExtensionResults = credential.getClientExtensionResults();
      }
      return out;
    }

    async function runWebAuthnLocalMFAVerify() {
      if (!window.PublicKeyCredential || !navigator.credentials || !navigator.credentials.get) {
        return null;
      }
      const start = await api('/auth/mfa/webauthn/verify/start', { method: 'POST', csrf: true });
      const challengeID = start && start.challenge_id ? String(start.challenge_id) : '';
      if (!challengeID) return null;
      const publicKey = normalizeWebAuthnRequestOptions(start.public_key || start);
      if (!publicKey) return null;
      const credential = await navigator.credentials.get({ publicKey });
      const response = serializeWebAuthnCredential(credential);
      return api(`/auth/mfa/webauthn/verify/finish?challenge_id=${encodeURIComponent(challengeID)}`, {
        method: 'POST',
        csrf: true,
        body: JSON.stringify(response),
      });
    }

    async function tryWebAuthnLocalMFAVerify() {
      const finish = await runWebAuthnLocalMFAVerify();
      if (!finish) return false;
      showApp(finish.user);
      return true;
    }

    async function runWebAuthnLocalMFARegister(displayName) {
      if (!window.PublicKeyCredential || !navigator.credentials || !navigator.credentials.create) {
        return null;
      }
      const start = await api('/auth/mfa/webauthn/register/start', { method: 'POST', csrf: true });
      const challengeID = start && start.challenge_id ? String(start.challenge_id) : '';
      if (!challengeID) return null;
      const publicKey = normalizeWebAuthnCreationOptions(start.public_key || start);
      if (!publicKey) return null;
      const credential = await navigator.credentials.create({ publicKey });
      const response = serializeWebAuthnCredential(credential);
      const cleanDisplayName = String(displayName || '').trim();
      const finishURL = `/auth/mfa/webauthn/register/finish?challenge_id=${encodeURIComponent(challengeID)}&display_name=${encodeURIComponent(cleanDisplayName)}`;
      return api(finishURL, {
        method: 'POST',
        csrf: true,
        body: JSON.stringify(response),
      });
    }

    async function tryWebAuthnLocalMFARegister() {
      const finish = await runWebAuthnLocalMFARegister('Passkey');
      if (!finish) return false;
      showApp(finish.user);
      return true;
    }

    function showLoginMFA(focus, context) {
      if (context === 'qr') {
        el.loginCodeField.classList.add('hidden');
      }
      el.localPasswordAction.classList.add('hidden');
      el.localPasswordField.classList.add('hidden');
      el.loginUseWebauthnBtn.classList.add('hidden');
      el.loginPasswordField.classList.remove('hidden');
      el.loginWithCodeBtn.classList.remove('hidden');
      el.loginPasswordLabel.textContent = 'Telegram two-step password (if required)';
      el.loginPasswordInput.placeholder = 'Telegram two-step password';
      if (focus) {
        el.loginPasswordInput.focus();
      }
    }

    function hasMethod(method) {
      return Array.isArray(state.localMFAMethods) && state.localMFAMethods.includes(method);
    }

    function resolveLocalCodeVerificationInput(rawCode) {
      const compact = String(rawCode || '').trim().replace(/\s+/g, '');
      if (!compact) {
        return { ok: false, message: 'Enter 2FA code or recovery key.' };
      }
      if (hasMethod('totp') && /^\d{6}$/.test(compact)) {
        return { ok: true, path: '/auth/mfa/totp/verify', code: compact };
      }
      if (hasMethod('recovery')) {
        return { ok: true, path: '/auth/mfa/recovery/verify', code: compact.toUpperCase() };
      }
      if (hasMethod('totp')) {
        return { ok: false, message: 'Enter a 6-digit 2FA code.' };
      }
      return { ok: false, message: 'No local 2FA code method is available.' };
    }

    function showLocalCodeVerification(focus) {
      state.loginMFAContext = 'local_code_verify';
      el.loginStatus.classList.remove('error');
      el.loginPhoneRow.classList.add('hidden');
      el.loginPasswordField.classList.add('hidden');
      el.localPasswordField.classList.add('hidden');
      resetLocalPasswordField();
      el.loginCodeField.classList.remove('hidden');
      el.loginWithCodeBtn.classList.remove('hidden');
      setLocalUnifiedCodeInputDefaults();
      if (focus) {
        el.loginCodeInput.focus();
      }
    }

    function showLocalPasswordVerification() {
      const hasCodeMethods = hasMethod('totp') || hasMethod('recovery');
      state.loginMFAContext = hasCodeMethods ? 'local_code_verify' : 'local_password_verify';
      el.loginStatus.classList.remove('error');
      el.loginPhoneRow.classList.add('hidden');
      el.loginPasswordField.classList.add('hidden');
      if (hasCodeMethods) {
        el.loginCodeField.classList.remove('hidden');
        setLocalUnifiedCodeInputDefaults();
      } else {
        el.loginCodeField.classList.add('hidden');
      }
      el.localPasswordField.classList.remove('hidden');
      el.loginWithCodeBtn.classList.remove('hidden');
      el.localPasswordInput.focus();
    }

    function toggleLocalPasswordVisibility() {
      const showPassword = el.localPasswordInput.type === 'password';
      el.localPasswordInput.type = showPassword ? 'text' : 'password';
      el.localPasswordToggleBtn.classList.toggle('visible', showPassword);
      const label = showPassword ? 'Hide local password' : 'Show local password';
      el.localPasswordToggleBtn.setAttribute('aria-label', label);
      el.localPasswordToggleBtn.setAttribute('title', label);
    }

    function renderLocalMFAActions() {
      el.loginUseWebauthnBtn.classList.toggle('hidden', !hasMethod('webauthn'));
      const allowPassword = hasMethod('password');
      el.localPasswordAction.classList.toggle('hidden', !allowPassword);
      if (!allowPassword) {
        el.localPasswordField.classList.add('hidden');
        resetLocalPasswordField();
      }
    }

    function isWebAuthnUserAgentDenied(err) {
      const name = String(err && err.name ? err.name : '').trim();
      if (name === 'NotAllowedError' || name === 'SecurityError') return true;
      const message = String(err && err.message ? err.message : '').toLowerCase();
      return message.includes('not allowed by the user agent')
        || message.includes('denied permission')
        || message.includes('operation either timed out')
        || message.includes('the operation was cancelled');
    }

    async function startLocalMFAFlow(errorResponse) {
      el.loginStatus.classList.remove('error');
      el.startQrBtn.classList.add('hidden');
      el.sendCodeBtn.classList.add('hidden');
      el.loginPhoneRow.classList.add('hidden');
      el.loginPasswordField.classList.add('hidden');
      el.loginPasswordInput.value = '';
      el.localPasswordField.classList.add('hidden');
      resetLocalPasswordField();
      el.loginCodeField.classList.remove('hidden');
      el.loginWithCodeBtn.classList.remove('hidden');
      const setupRequired = Boolean(errorResponse && errorResponse.payload && errorResponse.payload.setup_required);
      const methods = (errorResponse && errorResponse.payload && Array.isArray(errorResponse.payload.methods))
        ? errorResponse.payload.methods.slice()
        : [];
      state.localMFAMethods = methods;
      if (setupRequired) {
        try {
          const registered = await tryWebAuthnLocalMFARegister();
          if (registered) return;
        } catch (_) {}
        el.loginUseWebauthnBtn.classList.add('hidden');
        el.localPasswordAction.classList.add('hidden');
        const data = await api('/auth/mfa/totp/enroll/start', { method: 'POST', csrf: true });
        if (data && data.totp && data.totp.qr_image_url) {
          el.qrImage.src = data.totp.qr_image_url;
          el.qrImage.classList.remove('hidden');
        }
        state.loginMFAContext = 'local_totp_setup';
        el.loginCodeLabel.textContent = 'Local 2FA code';
        el.loginCodeInput.name = 'totp';
        el.loginCodeInput.autocomplete = 'one-time-code';
        el.loginCodeInput.inputMode = 'numeric';
        el.loginCodeInput.pattern = '[0-9]*';
        el.loginCodeInput.autocapitalize = 'none';
        el.loginCodeInput.spellcheck = false;
        el.loginCodeInput.placeholder = '123456';
        el.loginStatus.textContent = 'Scan QR code in authenticator app and enter local 2FA code.';
      } else {
        renderLocalMFAActions();
        if (hasMethod('totp') || hasMethod('recovery')) {
          showLocalCodeVerification(false);
        } else if (hasMethod('password')) {
          showLocalPasswordVerification();
        } else if (hasMethod('webauthn')) {
          state.loginMFAContext = '';
          el.loginPhoneRow.classList.add('hidden');
          el.loginCodeField.classList.add('hidden');
          el.loginPasswordField.classList.add('hidden');
          el.localPasswordField.classList.add('hidden');
          el.loginWithCodeBtn.classList.add('hidden');
        } else {
          throw new Error(humanizeError('telegram_login_required'));
        }
        el.qrImage.classList.add('hidden');
        el.qrImage.src = '';
        if (hasMethod('webauthn') && !hasMethod('totp') && !hasMethod('recovery') && !hasMethod('password')) {
          el.loginStatus.textContent = 'Use passkey to continue.';
        } else {
          el.loginStatus.textContent = 'Verify local 2FA to continue.';
        }
      }
      if (state.loginMFAContext !== 'local_password_verify') {
        el.loginCodeInput.value = '';
      }
    }

    function renderQRCountdown() {
      if (!state.qrExpiresAt) return;
      const remaining = Math.max(0, Math.floor((state.qrExpiresAt.getTime() - Date.now()) / 1000));
      if (remaining <= 0) {
        el.loginStatus.textContent = 'QR expired. Start QR again.';
        return;
      }
      const mm = String(Math.floor(remaining / 60)).padStart(2, '0');
      const ss = String(remaining % 60).padStart(2, '0');
      el.loginStatus.textContent = `QR expires in ${mm}:${ss}`;
    }

    function updateQR(qr) {
      state.qrLoginId = qr.id;
      state.qrExpiresAt = qr && qr.expires_at ? new Date(qr.expires_at) : null;
      if (qr.qr_image_url) {
        el.qrImage.src = qr.qr_image_url;
        el.qrImage.classList.remove('hidden');
      }
      clearInterval(state.qrCountdownTimer);
      state.qrCountdownTimer = setInterval(renderQRCountdown, 1000);
      renderQRCountdown();
    }

    async function pollQR() {
      if (!state.qrLoginId) return;
      try {
        const data = await api('/auth/telegram/qr/complete', {
          method: 'POST',
          body: JSON.stringify({ qr_login_id: state.qrLoginId }),
        });
        if (data.status === 'pending') {
          updateQR(data.qr_login);
          return;
        }
        clearInterval(state.qrTimer);
        clearInterval(state.qrCountdownTimer);
        el.qrImage.classList.add('hidden');
        showApp(data.user);
      } catch (err) {
        if (err && err.code === 'telegram_mfa_required') {
          clearInterval(state.qrTimer);
          clearInterval(state.qrCountdownTimer);
          state.loginMFAContext = 'qr';
          showLoginMFA(true, 'qr');
          el.qrImage.classList.add('hidden');
          el.loginStatus.textContent = 'Telegram two-step password required.';
          return;
        }
        clearInterval(state.qrTimer);
        clearInterval(state.qrCountdownTimer);
        el.loginStatus.classList.add('error');
        el.loginStatus.textContent = err.message;
      }
    }

    async function logout(forgetDevice) {
      const payload = { forget_device: Boolean(forgetDevice) };
      await api('/auth/logout', { method: 'POST', csrf: true, body: JSON.stringify(payload) });
      showLogin(forgetDevice ? 'Signed out and local device access removed.' : 'Signed out.');
      await loadRememberedAccount();
    }

    async function refreshFiles() {
      if (state.view === 'shared' && state.sharedRouteUnavailableMessage) {
        renderBreadcrumbs();
        renderSharedRouteUnavailable(state.sharedRouteUnavailableMessage);
        return;
      }
      el.filesBody.innerHTML = `<tr><td colspan="4" class="muted">Loading...</td></tr>`;
      renderBreadcrumbs();
      try {
        const data = state.view === 'shared'
          ? (state.currentFolderId
            ? await api(`/files?parent_id=${encodeURIComponent(state.currentFolderId)}`)
            : await api('/shared'))
          : await api(`/files${state.currentFolderId ? `?parent_id=${encodeURIComponent(state.currentFolderId)}` : ''}`);
        renderFiles(data.files || []);
      } catch (err) {
        el.filesBody.innerHTML = `<tr><td colspan="4" class="muted">${escapeHTML(err.message)}</td></tr>`;
      }
    }

    function sameFolderID(left, right) {
      return String(left || '') === String(right || '');
    }

    function isSelectableFile(file) {
      return Boolean(file) && !file.upload_id && !file.is_pending_upload;
    }

    function queueStatusForFileList(item) {
      const status = String(item && item.status ? item.status : '').trim();
      switch (status) {
        case 'queued':
        case 'hashing':
        case 'staging':
        case 'pending':
        case 'uploading':
          return status;
        case 'needs_file':
          return 'pending';
        case 'telegram':
        case 'completing':
          return 'uploading';
        default:
          return status || 'pending';
      }
    }

    function queueItemVisibleInFileList(item) {
      if (!item) return false;
      if (['complete', 'failed', 'canceled'].includes(item.status)) return false;
      return sameFolderID(item.parentID, state.currentFolderId || '');
    }

    function withQueuedUploads(serverFiles) {
      if (state.view !== 'own') return serverFiles;
      const existingUploadIDs = new Set(
        serverFiles.map((file) => String(file.upload_id || '').trim()).filter(Boolean),
      );
      const queuedRows = [];
      for (const item of state.uploadQueue) {
        if (!queueItemVisibleInFileList(item)) continue;
        const uploadID = String(item.uploadID || '').trim();
        if (uploadID && existingUploadIDs.has(uploadID)) continue;
        const fileName = item.file && item.file.name
          ? item.file.name
          : (item.displayPath || 'Uploading file');
        queuedRows.push({
          id: `queue:${item.id}`,
          upload_id: uploadID || `local:${item.id}`,
          is_pending_upload: true,
          type: 'file',
          name: fileName,
          status: queueStatusForFileList(item),
          plaintext_size: item.file && Number.isFinite(item.file.size) ? item.file.size : 0,
        });
      }
      return serverFiles.concat(queuedRows);
    }

    function renderFiles(files) {
      const serverFiles = Array.isArray(files) ? files.slice() : [];
      state.serverFiles = serverFiles;
      const displayFiles = withQueuedUploads(serverFiles);
      state.visibleFiles = displayFiles.slice();
      if (state.view !== 'own') {
        state.selectedFileIds = new Set();
        state.selectionAnchorID = '';
      } else {
        const visible = new Set(displayFiles.map((file) => file.id));
        state.selectedFileIds = new Set(Array.from(state.selectedFileIds).filter((id) => visible.has(id)));
        if (!visible.has(state.selectionAnchorID)) {
          state.selectionAnchorID = '';
        }
      }
      renderSelectionBar(displayFiles);
      if (!displayFiles.length) {
        const message = state.view === 'shared' ? 'No shared files.' : 'No files in this folder.';
        el.filesBody.innerHTML = `<tr><td colspan="4" class="muted">${message}</td></tr>`;
        if (state.view === 'own') wireFileSelectionHandlers([]);
        return;
      }
      el.filesBody.innerHTML = displayFiles.map((file) => `
        <tr draggable="${state.view === 'own' && !file.upload_id ? 'true' : 'false'}" data-file-id="${file.id}" data-file-type="${file.type}" data-file-name="${escapeHTML(file.name || 'Untitled')}" data-upload-id="${escapeHTML(file.upload_id || '')}">
          <td>${renderFileName(file)}</td>
          <td><span class="badge ${file.status === 'ready' ? 'ok' : 'warn'}">${escapeHTML(file.status)}</span></td>
          <td>${file.type === 'folder' ? '-' : formatBytes(file.plaintext_size || 0)}</td>
          <td><div class="actions row-actions">${renderFileActions(file)}</div></td>
        </tr>
      `).join('');
      wireFileSelectionHandlers(displayFiles);
      el.filesBody.querySelectorAll('[data-open-folder]').forEach((button) => {
        button.addEventListener('click', () => {
          const folder = displayFiles.find((file) => file.id === button.dataset.openFolder);
          if (!folder) return;
          if (hasActiveSelection()) {
            toggleFileSelection(folder.id, displayFiles);
            return;
          }
          navigateToFolder(folder);
        });
      });
      el.filesBody.querySelectorAll('[data-download]').forEach((button) => {
        button.addEventListener('click', async () => {
          const file = displayFiles.find((item) => item.id === button.dataset.download);
          if (!file) return;
          await downloadFile(file.id, file.name || 'download.bin');
        });
      });
      el.filesBody.querySelectorAll('[data-delete]').forEach((button) => {
        button.addEventListener('click', () => deleteFile(button.dataset.delete));
      });
      el.filesBody.querySelectorAll('[data-share]').forEach((button) => {
        button.addEventListener('click', () => {
          const file = displayFiles.find((item) => item.id === button.dataset.share);
          openShareDialog(file);
        });
      });
      el.filesBody.querySelectorAll('[data-details]').forEach((button) => {
        button.addEventListener('click', () => {
          const file = displayFiles.find((item) => item.id === button.dataset.details);
          openDetailsDialog(file);
        });
      });
      if (state.view === 'own') wireFileRowDragHandlers(displayFiles);
    }

    function renderSharedRouteUnavailable(message) {
      const text = escapeHTML(message || 'This shared item is unavailable or access is no longer active.');
      el.filesBody.innerHTML = `
        <tr>
          <td colspan="4">
            <div class="stack">
              <div class="muted">${text}</div>
              <div class="actions">
                <button id="sharedUnavailableBackBtn">Back to shared files</button>
              </div>
            </div>
          </td>
        </tr>
      `;
      const back = document.getElementById('sharedUnavailableBackBtn');
      if (back) {
        back.addEventListener('click', () => {
          state.sharedRouteUnavailableMessage = '';
          setHashRoute('#/shared');
        });
      }
    }

    function wireFileSelectionHandlers(files) {
      el.filesBody.querySelectorAll('[data-select-file]').forEach((input) => {
        input.addEventListener('change', (event) => {
          setFileSelection(input.dataset.selectFile, input.checked, files, { shiftKey: event.shiftKey });
        });
      });
      el.filesBody.querySelectorAll('[data-select-label]').forEach((label) => {
        label.addEventListener('click', (event) => {
          toggleFileSelection(label.dataset.selectLabel, files, { shiftKey: event.shiftKey });
        });
        label.addEventListener('keydown', (event) => {
          if (event.key !== 'Enter' && event.key !== ' ') return;
          event.preventDefault();
          toggleFileSelection(label.dataset.selectLabel, files, { shiftKey: event.shiftKey });
        });
      });
    }

    function selectableVisibleFileIDs(files) {
      return (Array.isArray(files) ? files : [])
        .filter((file) => isSelectableFile(file))
        .map((file) => file.id);
    }

    function setFileSelection(id, selected, files, options = {}) {
      if (!id || state.view !== 'own') return;
      const selectableIDs = selectableVisibleFileIDs(files);
      const targetIndex = selectableIDs.indexOf(id);
      if (targetIndex < 0) return;
      const shouldSelect = Boolean(selected);
      const anchorIndex = selectableIDs.indexOf(state.selectionAnchorID);
      const useRange = Boolean(options.shiftKey) && anchorIndex >= 0;
      if (useRange) {
        const start = Math.min(anchorIndex, targetIndex);
        const end = Math.max(anchorIndex, targetIndex);
        for (let idx = start; idx <= end; idx += 1) {
          const rowID = selectableIDs[idx];
          if (shouldSelect) state.selectedFileIds.add(rowID);
          else state.selectedFileIds.delete(rowID);
        }
      } else if (shouldSelect) {
        state.selectedFileIds.add(id);
      } else {
        state.selectedFileIds.delete(id);
      }
      state.selectionAnchorID = id;
      syncSelectionCheckboxes();
      renderSelectionBar(files);
    }

    function toggleFileSelection(id, files, options = {}) {
      if (!id || state.view !== 'own') return;
      setFileSelection(id, !state.selectedFileIds.has(id), files, options);
    }

    function hasActiveSelection() {
      return state.view === 'own' && state.selectedFileIds.size > 0;
    }

    function wireFileRowDragHandlers(files) {
      el.filesBody.querySelectorAll('tr[data-file-id]').forEach((row) => {
        if (row.dataset.uploadId) return;
        row.addEventListener('dragstart', (event) => {
          const selected = Array.from(state.selectedFileIds);
          const rowID = row.dataset.fileId;
          const wasSelected = state.selectedFileIds.has(rowID);
          if (!wasSelected) {
            state.selectedFileIds = new Set([rowID]);
          }
          state.draggingItems = wasSelected && selected.length > 1
            ? selected
            : [rowID];
          renderSelectionBar(files);
          syncSelectionCheckboxes();
          event.dataTransfer.effectAllowed = 'move';
          event.dataTransfer.setData('application/x-televault-file-ids', JSON.stringify(state.draggingItems));
          event.dataTransfer.setData('application/x-televault-file-id', row.dataset.fileId);
          event.dataTransfer.setData('text/plain', row.dataset.fileId);
        });
        row.addEventListener('dragend', () => {
          state.draggingItems = [];
          el.filesBody.querySelectorAll('.drag-over').forEach((item) => item.classList.remove('drag-over'));
          el.breadcrumbs.querySelectorAll('.drag-over').forEach((item) => item.classList.remove('drag-over'));
          el.upBtn.classList.remove('drag-over');
        });
        if (row.dataset.fileType !== 'folder') return;
        row.addEventListener('dragover', (event) => {
          if (state.draggingItems.includes(row.dataset.fileId)) return;
          if (!state.draggingItems.length && !hasExternalFiles(event.dataTransfer)) return;
          event.preventDefault();
          event.dataTransfer.dropEffect = state.draggingItems.length ? 'move' : 'copy';
          row.classList.add('drag-over');
        });
        row.addEventListener('dragleave', () => row.classList.remove('drag-over'));
        row.addEventListener('drop', async (event) => {
          event.preventDefault();
          event.stopPropagation();
          row.classList.remove('drag-over');
          if (state.draggingItems.length) {
            const ids = state.draggingItems.filter((id) => id !== row.dataset.fileId);
            if (!ids.length) return;
            await moveFiles(ids, row.dataset.fileId);
            return;
          }
          const queued = await enqueueDroppedStructure(event.dataTransfer, row.dataset.fileId);
          if (!queued) return;
          setUploadStatus(`Queued ${queued} file${queued === 1 ? '' : 's'} for this folder.`);
          runUploadQueue();
        });
      });
    }

    function renderFileActions(file) {
      if (file.upload_id || file.is_pending_upload) {
        return `<span class="muted">In progress</span>`;
      }
      const readOnly = Boolean(state.readOnlyMapMode);
      if (state.view === 'shared') {
        const deleteButton = file.can_delete
          ? `<button data-delete="${file.id}" ${readOnly ? 'disabled title="Read-only mode"' : ''}>Delete</button>`
          : '';
        if (file.type === 'folder') {
          return `<button data-open-folder="${file.id}">Open</button><button data-details="${file.id}">Details</button>${deleteButton}`;
        }
        return `<button data-download="${file.id}">Download</button><button data-details="${file.id}">Details</button>${deleteButton}`;
      }
      const downloadButton = `<button data-download="${file.id}">Download</button>`;
      const deleteButton = `<button data-delete="${file.id}" ${readOnly ? 'disabled title="Read-only mode"' : ''}>Delete</button>`;
      if (file.type === 'folder') {
        return `<button data-open-folder="${file.id}">Open</button><button data-details="${file.id}">Details</button>${deleteButton}`;
      }
      return `${downloadButton}<button data-details="${file.id}">Details</button><button data-share="${file.id}" ${readOnly ? 'disabled title="Read-only mode"' : ''}>Share</button>${deleteButton}`;
    }

    function downloadFileNameFromHeader(headerValue, fallbackName) {
      const fallback = String(fallbackName || 'download.bin');
      const raw = String(headerValue || '').trim();
      if (!raw) return fallback;
      const utf8Match = raw.match(/filename\*\s*=\s*UTF-8''([^;]+)/i);
      if (utf8Match && utf8Match[1]) {
        try {
          return decodeURIComponent(utf8Match[1]);
        } catch (_) {
          return utf8Match[1];
        }
      }
      const quotedMatch = raw.match(/filename\s*=\s*"([^"]+)"/i);
      if (quotedMatch && quotedMatch[1]) return quotedMatch[1];
      const plainMatch = raw.match(/filename\s*=\s*([^;]+)/i);
      if (plainMatch && plainMatch[1]) return plainMatch[1].trim();
      return fallback;
    }

    async function downloadFile(id, fallbackName) {
      if (!id) return;
      if (!requireWritableAction()) return;
      try {
        const response = await fetch(`/files/${encodeURIComponent(id)}/download`, {
          credentials: 'same-origin',
        });
        if (!response.ok) {
          let code = 'download_failed';
          try {
            const payload = await response.json();
            code = payload && payload.error ? String(payload.error) : code;
          } catch (_) {
            code = 'download_failed';
          }
          throw new Error(humanizeError(code));
        }
        const blob = await response.blob();
        const fileName = downloadFileNameFromHeader(response.headers.get('Content-Disposition'), fallbackName);
        const url = URL.createObjectURL(blob);
        try {
          const link = document.createElement('a');
          link.href = url;
          link.download = fileName;
          document.body.appendChild(link);
          link.click();
          link.remove();
        } finally {
          URL.revokeObjectURL(url);
        }
        setUploadStatus(`Downloaded ${fileName}.`);
      } catch (err) {
        const raw = String((err && err.message) || '').trim();
        if (raw.includes('NetworkError') || raw.includes('Failed to fetch')) {
          setUploadStatus(`${humanizeError('download_failed')} ${humanizeError('telegram_session_stale')}`, true);
          return;
        }
        setUploadStatus(raw || humanizeError('download_failed'), true);
      }
    }

    function renderFileName(file) {
      const name = escapeHTML(file.name || 'Untitled');
      const select = state.view === 'own' && !file.upload_id
        ? `<label class="file-select"><input type="checkbox" data-select-file="${file.id}" ${state.selectedFileIds.has(file.id) ? 'checked' : ''}></label>`
        : '';
      const selectLabel = state.view === 'own' && !file.upload_id ? ` data-select-label="${file.id}" role="button" tabindex="0"` : '';
      const label = file.type !== 'folder' || file.upload_id
        ? `<span class="file-label"${selectLabel}>${name}</span>`
        : `<button type="button" class="link folder-name file-label" data-open-folder="${file.id}"><span class="folder-mark" aria-hidden="true"></span>${name}</button>`;
      return `<span class="file-cell">${select}${label}</span>`;
    }

    function renderSelectionBar(files) {
      const selectedCount = state.selectedFileIds.size;
      const visibleCount = files.filter((file) => !(file.upload_id || file.is_pending_upload)).length;
      const shouldShow = state.view === 'own' && selectedCount > 0;
      el.selectionBar.classList.toggle('hidden', !shouldShow);
      el.selectionSummary.textContent = selectedCount === 1
        ? '1 item selected.'
        : `${selectedCount} items selected.`;
      el.moveSelectedBtn.disabled = selectedCount === 0;
      el.deleteSelectedBtn.disabled = selectedCount === 0;
      el.clearSelectionBtn.disabled = selectedCount === 0;
      el.selectAllVisibleBtn.disabled = visibleCount === 0;
      el.selectAllVisibleBtn.textContent = selectedCount === visibleCount && visibleCount > 0 ? 'Deselect all' : 'Select all';
    }

    function renderBreadcrumbs() {
      const rootLabel = state.view === 'shared' ? 'Shared with me' : 'Root';
      const root = `<button class="link" data-crumb-index="-1">${rootLabel}</button>`;
      const crumbs = state.folderStack.map((folder, index) => (
        `<span>/</span><button class="link" data-crumb-index="${index}">${escapeHTML(folder.name || 'Untitled')}</button>`
      )).join('');
      el.breadcrumbs.innerHTML = root + crumbs;
      el.upBtn.disabled = !state.currentFolderId;
      el.ownFilesBtn.classList.toggle('active', state.view === 'own');
      el.sharedFilesBtn.classList.toggle('active', state.view === 'shared');
      el.breadcrumbs.querySelectorAll('[data-crumb-index]').forEach((button) => {
        button.addEventListener('click', () => jumpToCrumb(Number(button.dataset.crumbIndex)));
      });
      wireAncestorDropTargets();
    }

    function wireAncestorDropTargets() {
      el.breadcrumbs.querySelectorAll('[data-crumb-index]').forEach((button) => {
        button.addEventListener('dragover', (event) => {
          if (!state.draggingItems.length || state.view !== 'own') return;
          event.preventDefault();
          event.dataTransfer.dropEffect = 'move';
          button.classList.add('drag-over');
        });
        button.addEventListener('dragleave', () => button.classList.remove('drag-over'));
        button.addEventListener('drop', async (event) => {
          if (!state.draggingItems.length || state.view !== 'own') return;
          event.preventDefault();
          button.classList.remove('drag-over');
          const index = Number(button.dataset.crumbIndex);
          const targetID = index < 0 ? '' : (state.folderStack[index] ? state.folderStack[index].id : '');
          await moveFiles(state.draggingItems, targetID);
        });
      });
    }

    function navigateToFolder(folder) {
      if (!folder || folder.type !== 'folder') return;
      state.sharedRouteUnavailableMessage = '';
      state.currentFolderId = folder.id;
      state.folderStack.push({ id: folder.id, name: folder.name || 'Untitled' });
      saveFolderState();
      if (state.view === 'shared') {
        setHashRoute(sharedFolderRoute(folder.id));
        return;
      }
      setHashRoute(folderRoute(folder.id));
    }

    function openFolder(folder) {
      if (!folder || folder.type !== 'folder') return;
      state.sharedRouteUnavailableMessage = '';
      state.currentFolderId = folder.id;
      state.folderStack.push({ id: folder.id, name: folder.name || 'Untitled' });
      saveFolderState();
      refreshFiles();
    }

    function goUp() {
      if (!state.currentFolderId) return;
      state.sharedRouteUnavailableMessage = '';
      state.folderStack.pop();
      const parent = state.folderStack[state.folderStack.length - 1];
      state.currentFolderId = parent ? parent.id : '';
      saveFolderState();
      if (state.view === 'shared') {
        setHashRoute(parent ? sharedFolderRoute(parent.id) : '#/shared');
        return;
      }
      setHashRoute(parent ? folderRoute(parent.id) : '#/');
    }

    function jumpToCrumb(index) {
      state.sharedRouteUnavailableMessage = '';
      if (index < 0) {
        state.folderStack = [];
        state.currentFolderId = '';
      } else {
        state.folderStack = state.folderStack.slice(0, index + 1);
        state.currentFolderId = state.folderStack[index].id;
      }
      saveFolderState();
      if (state.view === 'shared') {
        setHashRoute(state.currentFolderId ? sharedFolderRoute(state.currentFolderId) : '#/shared');
        return;
      }
      setHashRoute(state.currentFolderId ? folderRoute(state.currentFolderId) : '#/');
    }

    function setView(view) {
      state.view = view;
      state.droppedFiles = [];
      state.currentFolderId = '';
      state.folderStack = [];
      state.sharedRouteUnavailableMessage = '';
      saveFolderState();
      setHashRoute(view === 'shared' ? '#/shared' : '#/');
    }

    function setHashRoute(hash) {
      if (window.location.hash === hash) {
        applyRoute();
        return;
      }
      window.location.hash = hash;
    }

    function folderRoute(id) {
      return `#/folder/${encodeURIComponent(id)}`;
    }

    function sharedFolderRoute(id) {
      return `#/shared/folder/${encodeURIComponent(id)}`;
    }

    async function applyRoute() {
      if (!state.user || state.applyingRoute) return;
      state.applyingRoute = true;
      try {
        const route = parseHashRoute();
        if (route.view === 'shared') {
          state.view = 'shared';
          state.droppedFiles = [];
          if (!route.folderId) {
            state.currentFolderId = '';
            state.folderStack = [];
            state.sharedRouteUnavailableMessage = '';
            renderBreadcrumbs();
            await refreshFiles();
            return;
          }
          try {
            await loadSharedFolderRoute(route.folderId);
            state.sharedRouteUnavailableMessage = '';
          } catch (err) {
            state.currentFolderId = '';
            state.folderStack = [];
            state.sharedRouteUnavailableMessage = 'This shared item is unavailable or access is no longer active.';
            if (err && err.code && err.code !== 'file_not_found') {
              state.sharedRouteUnavailableMessage = humanizeError(err.code);
            }
          }
          renderBreadcrumbs();
          await refreshFiles();
          return;
        }

        state.view = 'own';
        state.droppedFiles = [];
        state.sharedRouteUnavailableMessage = '';
        if (!route.folderId) {
          state.currentFolderId = '';
          state.folderStack = [];
          saveFolderState();
          renderBreadcrumbs();
          await refreshFiles();
          return;
        }

        try {
          await loadFolderRoute(route.folderId);
        } catch (err) {
          state.currentFolderId = '';
          state.folderStack = [];
          saveFolderState();
          if (window.location.hash !== '#/') {
            window.location.hash = '#/';
            return;
          }
          setUploadStatus(err.message, true);
        }
        saveFolderState();
        renderBreadcrumbs();
        await refreshFiles();
      } finally {
        state.applyingRoute = false;
      }
    }

    function parseHashRoute() {
      const hash = window.location.hash || '#/';
      if (hash === '#/shared') return { view: 'shared', folderId: '' };
      const sharedMatch = hash.match(/^#\/shared\/folder\/([^/]+)$/);
      if (sharedMatch) return { view: 'shared', folderId: decodeURIComponent(sharedMatch[1]) };
      const match = hash.match(/^#\/folder\/([^/]+)$/);
      if (match) return { view: 'own', folderId: decodeURIComponent(match[1]) };
      return { view: 'own', folderId: '' };
    }

    async function loadFolderRoute(folderId) {
      const stack = [];
      let currentID = folderId;
      const seen = new Set();
      while (currentID) {
        if (seen.has(currentID)) throw new Error('folder_cycle');
        seen.add(currentID);
        const data = await api(`/files/${encodeURIComponent(currentID)}`);
        const folder = data.file;
        if (!folder || folder.type !== 'folder') throw new Error('folder_not_found');
        stack.unshift({ id: folder.id, name: folder.name || 'Untitled' });
        currentID = folder.parent_id || '';
      }
      state.currentFolderId = folderId;
      state.folderStack = stack;
    }

    async function loadSharedFolderRoute(folderId) {
      const stack = [];
      let currentID = folderId;
      const seen = new Set();
      while (currentID) {
        if (seen.has(currentID)) throw new Error('folder_cycle');
        seen.add(currentID);
        let folder;
        try {
          const data = await api(`/files/${encodeURIComponent(currentID)}`);
          folder = data.file;
        } catch (err) {
          if (stack.length === 0) throw err;
          break;
        }
        if (!folder || folder.type !== 'folder') throw new Error('folder_not_found');
        stack.unshift({ id: folder.id, name: folder.name || 'Untitled' });
        currentID = folder.parent_id || '';
      }
      state.currentFolderId = folderId;
      state.folderStack = stack;
    }

    async function createFolder() {
      if (!requireWritableAction()) return;
      const name = el.folderNameInput.value.trim();
      if (!name) return;
      el.createFolderBtn.disabled = true;
      try {
        await api('/folders', {
          method: 'POST',
          csrf: true,
          body: JSON.stringify({
            name,
            parent_id: state.currentFolderId || undefined,
          }),
        });
        el.folderNameInput.value = '';
        await applyRoute();
      } catch (err) {
        setUploadStatus(err.message, true);
      } finally {
        el.createFolderBtn.disabled = false;
      }
    }

    async function deleteFile(id) {
      if (!requireWritableAction()) return;
      if (!id) return;
      const file = state.visibleFiles.find((item) => item.id === id);
      const isSharedDelete = state.view === 'shared' && file && file.access === 'shared_read_delete';
      const message = isSharedDelete
        ? 'Delete this shared item for everyone?'
        : 'Delete this item?';
      if (!window.confirm(message)) return;
      await deleteFiles([id], false);
    }

    async function deleteFiles(ids, confirmDelete = true) {
      if (!requireWritableAction()) return;
      const normalized = Array.from(new Set((ids || []).map((id) => String(id).trim()).filter(Boolean)));
      if (!normalized.length) return;
      if (confirmDelete && !window.confirm(`Delete ${normalized.length} selected item${normalized.length === 1 ? '' : 's'}?`)) return;
      try {
        if (normalized.length === 1) {
          await api(`/files/${normalized[0]}`, { method: 'DELETE', csrf: true });
        } else {
          await api('/files/bulk-delete', {
            method: 'POST',
            csrf: true,
            body: JSON.stringify({ ids: normalized }),
          });
        }
        state.selectedFileIds.clear();
        state.draggingItems = [];
        await applyRoute();
      } catch (err) {
        setUploadStatus(err.message, true);
      }
    }

    async function moveFile(id, parentID) {
      await moveFiles([id], parentID);
    }

    async function openMoveDialog() {
      if (!requireWritableAction(setMoveStatus)) return;
      const selected = Array.from(state.selectedFileIds);
      if (!selected.length || state.view !== 'own') return;
      setMoveStatus('');
      el.moveSummary.textContent = `${selected.length} item${selected.length === 1 ? '' : 's'} selected`;
      el.confirmMoveBtn.disabled = true;
      el.moveTargetSelect.disabled = true;
      el.moveTargetSelect.innerHTML = '<option value="">Loading folders...</option>';
      el.moveModal.classList.remove('hidden');
      try {
        state.movePickerFolders = await loadAllOwnFolders();
        renderMoveTargets(selected);
      } catch (err) {
        el.moveTargetSelect.innerHTML = '<option value="">Folder list unavailable</option>';
        setMoveStatus(err.message, true);
      }
    }

    function closeMoveDialog() {
      el.moveModal.classList.add('hidden');
      state.movePickerFolders = [];
      setMoveStatus('');
    }

    async function confirmMoveSelected() {
      const selected = Array.from(state.selectedFileIds);
      if (!selected.length || state.view !== 'own') return;
      const targetID = String(el.moveTargetSelect.value || '');
      if (!targetID && targetID !== '') return;
      el.confirmMoveBtn.disabled = true;
      try {
        await moveFiles(selected, targetID);
        closeMoveDialog();
      } catch (_) {
        // moveFiles already sets a status message.
      } finally {
        el.confirmMoveBtn.disabled = false;
      }
    }

    async function loadAllOwnFolders() {
      const folders = [{ id: '', path: 'Root' }];
      const queue = [{ id: '', path: 'Root' }];
      const seen = new Set(['']);
      while (queue.length) {
        const current = queue.shift();
        const data = await api(`/files${current.id ? `?parent_id=${encodeURIComponent(current.id)}` : ''}`);
        const list = (data.files || []).filter((item) => item.type === 'folder');
        for (const folder of list) {
          if (!folder.id || seen.has(folder.id)) continue;
          seen.add(folder.id);
          const path = `${current.path}/${folder.name || 'Untitled'}`;
          const entry = { id: folder.id, path };
          folders.push(entry);
          queue.push(entry);
        }
      }
      return folders;
    }

    function renderMoveTargets(selectedIDs) {
      const forbidden = new Set(selectedIDs.map((id) => String(id)));
      const options = state.movePickerFolders
        .filter((folder) => !forbidden.has(folder.id))
        .map((folder) => `<option value="${escapeHTML(folder.id)}">${escapeHTML(folder.path)}</option>`);
      el.moveTargetSelect.innerHTML = options.join('');
      if (!options.length) {
        el.moveTargetSelect.innerHTML = '<option value="">No valid destination</option>';
        el.confirmMoveBtn.disabled = true;
        el.moveTargetSelect.disabled = true;
        return;
      }
      el.moveTargetSelect.disabled = false;
      el.confirmMoveBtn.disabled = false;
      el.moveTargetSelect.value = state.currentFolderId || '';
    }

    function setMoveStatus(message, error = false) {
      el.moveStatus.textContent = message;
      el.moveStatus.classList.toggle('error', error);
    }

    async function moveFiles(ids, parentID) {
      if (!requireWritableAction()) return;
      const normalized = Array.from(new Set((ids || []).map((id) => String(id).trim()).filter(Boolean)));
      if (!normalized.length) return;
      try {
        await api('/files/bulk-move', {
          method: 'PATCH',
          csrf: true,
          body: JSON.stringify({
            ids: normalized,
            parent_id: parentID || '',
          }),
        });
        setUploadStatus('Item moved.');
        state.selectedFileIds.clear();
        state.draggingItems = [];
        await applyRoute();
      } catch (err) {
        setUploadStatus(err.message, true);
        throw err;
      }
    }

    async function renameFile(id, name) {
      if (!requireWritableAction(setDetailsStatus)) return;
      const normalized = String(name || '').trim();
      if (!id || !normalized) return;
      try {
        await api(`/files/${id}`, {
          method: 'PATCH',
          csrf: true,
          body: JSON.stringify({ name: normalized }),
        });
        setDetailsStatus('Item renamed.');
        if (state.detailsFile && state.detailsFile.id === id) {
          state.detailsFile.name = normalized;
        }
        el.detailsFileName.textContent = normalized;
        el.detailsFileNameInput.value = normalized;
        await applyRoute();
        if (state.detailsFile && state.detailsFile.id === id) {
          closeDetailsDialog();
        }
      } catch (err) {
        setDetailsStatus(err.message, true);
      }
    }

    async function openShareDialog(file) {
      if (!requireWritableAction(setUploadStatus)) return;
      if (!file) return;
      closeDetailsDialog();
      state.shareFile = file;
      state.shareRecipients = [];
      el.shareFileName.textContent = file.name || 'Untitled';
      el.shareRecipientSelect.innerHTML = '<option value="">Loading...</option>';
      el.shareRecipientSelect.value = '';
      el.shareRecipientHint.textContent = 'Only registered TeleVault users from your Telegram contacts/chats are listed.';
      toggleShareManualInput(false);
      el.shareTelegramInput.value = '';
      el.sharePermissionSelect.value = 'read';
      el.shareExpirySelect.value = '';
      el.publicExpirySelect.value = '1d';
      el.publicLinkInput.value = '';
      el.publicPasswordInput.value = '';
      el.publicDownloadLimitInput.value = '';
      el.publicDownloadLimitModeSelect.value = 'hard';
      el.publicLinkResult.classList.add('hidden');
      setShareStatus('');
      showShareTab('internal');
      el.shareModal.classList.remove('hidden');
      await refreshShareRecipients();
      await refreshShares();
      await refreshPublicLinks();
      focusShareTabInput(state.shareTab);
    }

    function closeShareDialog() {
      state.shareFile = null;
      el.shareModal.classList.add('hidden');
    }

    function focusShareTabInput(tab) {
      if (tab === 'public') {
        el.publicExpirySelect.focus();
        return;
      }
      if (el.shareManualField.classList.contains('hidden')) {
        el.shareRecipientSelect.focus();
        return;
      }
      el.shareTelegramInput.focus();
    }

    function showShareTab(tab) {
      const nextTab = tab === 'public' ? 'public' : 'internal';
      state.shareTab = nextTab;
      const internalActive = nextTab === 'internal';
      el.shareInternalTabBtn.classList.toggle('active', internalActive);
      el.sharePublicTabBtn.classList.toggle('active', !internalActive);
      el.shareInternalTabBtn.setAttribute('aria-selected', internalActive ? 'true' : 'false');
      el.sharePublicTabBtn.setAttribute('aria-selected', internalActive ? 'false' : 'true');
      el.shareInternalTabPanel.classList.toggle('hidden', !internalActive);
      el.sharePublicTabPanel.classList.toggle('hidden', internalActive);
    }

    async function openDetailsDialog(file) {
      if (!file) return;
      closeShareDialog();
      state.detailsRequestID += 1;
      const requestID = state.detailsRequestID;
      state.detailsFile = file;
      state.detailsDownloadActivity = null;
      el.detailsFileName.textContent = file.name || 'Untitled';
      el.detailsFileID.value = file.id || '';
      setDetailsStatus('Loading...');
      renderDetails(file);
      el.detailsModal.classList.remove('hidden');
      try {
        const [detailsRes, activityRes] = await Promise.all([
          api(`/files/${file.id}`),
          api(`/files/${file.id}/activity`).catch(() => ({ download_activity: null })),
        ]);
        if (state.detailsRequestID !== requestID) return;
        const details = detailsRes.file || file;
        state.detailsDownloadActivity = activityRes.download_activity || null;
        state.detailsFile = details;
        renderDetails(details);
        setDetailsStatus('');
      } catch (err) {
        if (state.detailsRequestID !== requestID) return;
        renderDetails(file);
        setDetailsStatus(err.message, true);
      }
    }

    function closeDetailsDialog() {
      state.detailsFile = null;
      state.detailsDownloadActivity = null;
      state.detailsRequestID += 1;
      el.detailsModal.classList.add('hidden');
    }

    async function refreshShares() {
      if (!state.shareFile) return;
      el.sharesBody.innerHTML = `<tr><td colspan="4" class="muted">Loading...</td></tr>`;
      try {
        const data = await api(`/files/${state.shareFile.id}/shares`);
        renderShares(data.shares || []);
      } catch (err) {
        el.sharesBody.innerHTML = `<tr><td colspan="4" class="muted">${escapeHTML(err.message)}</td></tr>`;
      }
    }

    function renderShareRecipients(recipients) {
      if (!Array.isArray(recipients) || recipients.length === 0) {
        state.shareRecipients = [];
        el.shareRecipientSelect.innerHTML = '<option value="">No matching users found</option>';
        el.shareRecipientSelect.value = '';
        el.shareRecipientHint.textContent = 'Internal share works only for users who already logged into this TeleVault instance.';
        return;
      }
      state.shareRecipients = recipients;
      el.shareRecipientSelect.innerHTML = '<option value="">Select recipient</option>' + recipients.map((recipient) => {
        const displayName = String(recipient.display_name || '').trim();
        const username = String(recipient.username || '').trim();
        const label = displayName || username || `Telegram ${recipient.telegram_id}`;
        const subtitle = username ? ` @${username}` : '';
        return `<option value="${escapeHTML(String(recipient.telegram_id))}">${escapeHTML(label + subtitle)}</option>`;
      }).join('');
      el.shareRecipientHint.textContent = 'Only registered TeleVault users from your Telegram contacts/chats are listed.';
    }

    async function refreshShareRecipients() {
      el.shareRecipientSelect.innerHTML = '<option value="">Loading...</option>';
      el.shareRecipientHint.textContent = 'Loading recipients...';
      try {
        const data = await api('/share-recipients');
        renderShareRecipients(data.recipients || []);
      } catch (err) {
        state.shareRecipients = [];
        el.shareRecipientSelect.innerHTML = `<option value="">${escapeHTML(err.message)}</option>`;
        el.shareRecipientHint.textContent = 'Recipient discovery is unavailable right now.';
      }
    }

    function toggleShareManualInput(forceShow) {
      const showManual = typeof forceShow === 'boolean'
        ? forceShow
        : el.shareManualField.classList.contains('hidden');
      el.shareManualField.classList.toggle('hidden', !showManual);
      el.shareManualToggleBtn.textContent = showManual ? 'Use recipient list' : 'Enter ID manually';
      if (!showManual) {
        el.shareTelegramInput.value = '';
      }
    }

    async function refreshPublicLinks() {
      if (!state.shareFile) return;
      el.publicLinksBody.innerHTML = `<tr><td colspan="4" class="muted">Loading...</td></tr>`;
      try {
        const data = await api(`/files/${state.shareFile.id}/public-links`);
        renderPublicLinks(data.public_links || []);
      } catch (err) {
        el.publicLinksBody.innerHTML = `<tr><td colspan="4" class="muted">${escapeHTML(err.message)}</td></tr>`;
      }
    }

    function renderShares(shares) {
      if (!shares.length) {
        el.sharesBody.innerHTML = `<tr><td colspan="4" class="muted">No active shares.</td></tr>`;
        return;
      }
      el.sharesBody.innerHTML = shares.map((share) => `
        <tr>
          <td>${escapeHTML(share.grantee_username || share.grantee_name || `Telegram ${share.grantee_telegram_id}`)}</td>
          <td>${escapeHTML(sharePermissionLabel(share.permission))}</td>
          <td>${share.expires_at ? escapeHTML(new Date(share.expires_at).toLocaleString()) : 'No limit'}</td>
          <td><button data-revoke-share="${share.id}">Revoke</button></td>
        </tr>
      `).join('');
      el.sharesBody.querySelectorAll('[data-revoke-share]').forEach((button) => {
        button.addEventListener('click', () => revokeShare(button.dataset.revokeShare));
      });
    }

    async function createShare() {
      if (!requireWritableAction(setShareStatus)) return;
      if (!state.shareFile) return;
      const useManual = !el.shareManualField.classList.contains('hidden');
      const selectedID = Number(el.shareRecipientSelect.value);
      const manualID = Number(el.shareTelegramInput.value);
      const telegramID = useManual ? manualID : selectedID;
      if (!Number.isInteger(telegramID) || telegramID <= 0) {
        setShareStatus(useManual ? 'telegram_id_required' : 'share_recipient_required', true);
        return;
      }
      try {
        el.createShareBtn.disabled = true;
        await api(`/files/${state.shareFile.id}/shares`, {
          method: 'POST',
          csrf: true,
          body: JSON.stringify({
            telegram_id: telegramID,
            permission: String(el.sharePermissionSelect.value || 'read'),
            expires_at: selectedShareExpiry(),
          }),
        });
        if (useManual) {
          el.shareTelegramInput.value = '';
        } else {
          el.shareRecipientSelect.value = '';
        }
        el.shareExpirySelect.value = '';
        setShareStatus('Share created.');
        await refreshShares();
      } catch (err) {
        setShareStatus(err.message, true);
      } finally {
        el.createShareBtn.disabled = false;
      }
    }

    async function revokeShare(shareID) {
      if (!requireWritableAction(setShareStatus)) return;
      if (!state.shareFile || !shareID) return;
      try {
        await api(`/files/${state.shareFile.id}/shares/${shareID}`, { method: 'DELETE', csrf: true });
        setShareStatus('Share revoked.');
        await refreshShares();
      } catch (err) {
        setShareStatus(err.message, true);
      }
    }

    async function createPublicLink() {
      if (!requireWritableAction(setShareStatus)) return;
      if (!state.shareFile) return;
      const password = String(el.publicPasswordInput.value || '').trim();
      const minLength = getPublicLinkPasswordMinLength();
      if (password && password.length < minLength) {
        setShareStatus(`Password must be at least ${minLength} characters.`, true);
        return;
      }
      const rawDownloadLimit = String(el.publicDownloadLimitInput.value || '').trim();
      if (rawDownloadLimit !== '' && (!/^\d+$/.test(rawDownloadLimit) || Number(rawDownloadLimit) <= 0)) {
        setShareStatus('Download limit must be a positive integer.', true);
        return;
      }
      try {
        el.createPublicLinkBtn.disabled = true;
        const data = await api(`/files/${state.shareFile.id}/public-links`, {
          method: 'POST',
          csrf: true,
          body: JSON.stringify({
            expires_at: selectedExpiry(el.publicExpirySelect.value),
            password: password || undefined,
            max_downloads: rawDownloadLimit === '' ? null : Number(rawDownloadLimit),
            download_limit_mode: el.publicDownloadLimitModeSelect.value || 'hard',
            show_checksum: Boolean(el.publicShowChecksumInput.checked),
          }),
        });
        el.publicLinkInput.value = data.url || '';
        el.publicPasswordInput.value = '';
        el.publicDownloadLimitInput.value = '';
        el.publicDownloadLimitModeSelect.value = 'hard';
        el.publicShowChecksumInput.checked = false;
        el.publicLinkResult.classList.remove('hidden');
        showShareTab('public');
        setShareStatus('Public link created.');
        await refreshPublicLinks();
      } catch (err) {
        setShareStatus(err.message, true);
      } finally {
        el.createPublicLinkBtn.disabled = false;
      }
    }

    function renderPublicLinks(links) {
      if (!links.length) {
        el.publicLinksBody.innerHTML = `<tr><td colspan="4" class="muted">No active public links.</td></tr>`;
        return;
      }
      el.publicLinksBody.innerHTML = links.map((link) => `
        <tr>
          <td>${escapeHTML(new Date(link.created_at).toLocaleString())}</td>
          <td>${link.expires_at ? escapeHTML(new Date(link.expires_at).toLocaleString()) : 'No limit'}${link.password_required ? ' + password' : ''}${link.show_checksum ? ' + SHA-256' : ''}</td>
          <td>${Number.isInteger(link.max_downloads) ? `${link.download_count || 0} / ${link.max_downloads}${link.download_limit_mode === 'hard' && Number.isInteger(link.active_download_count) && link.active_download_count > 0 ? ` (${link.active_download_count} active)` : ''}` : 'Unlimited'}${link.download_limit_mode ? ` (${escapeHTML(String(link.download_limit_mode))})` : ''}</td>
          <td><button data-revoke-public-link="${link.id}">Revoke</button></td>
        </tr>
      `).join('');
      el.publicLinksBody.querySelectorAll('[data-revoke-public-link]').forEach((button) => {
        button.addEventListener('click', () => revokePublicLink(button.dataset.revokePublicLink));
      });
    }

    function renderDetails(file) {
      if (!file) {
        el.detailsBody.innerHTML = '';
        el.detailsFileName.textContent = '';
        el.detailsFileID.value = '';
        el.detailsFileNameInput.value = '';
        el.detailsFileNameInput.disabled = true;
        el.saveDetailsFileNameBtn.disabled = true;
        return;
      }
      el.detailsFileName.textContent = file.name || 'Untitled';
      el.detailsFileID.value = file.id || '';
      el.detailsFileNameInput.value = file.name || '';
      const editable = String(file.access || '') === 'owner' || Boolean(state.user && file.owner_id === state.user.id);
      el.detailsFileNameInput.disabled = !editable;
      el.saveDetailsFileNameBtn.disabled = !editable;
      const rows = [];
      const ownerLabel = formatOwnerLabel(file);
      const accessLabel = formatAccessLabel(file, editable);
      rows.push(['Type', file.type || '-']);
      rows.push(['Status', file.status || '-']);
      rows.push(['Owner', ownerLabel]);
      rows.push(['Access', accessLabel]);
      rows.push(['Size', file.type === 'file' ? formatBytes(file.plaintext_size || 0) : '-']);
      rows.push(['MIME', file.type === 'file' ? (file.mime_type || '-') : '-']);
      rows.push([file.type === 'file' ? 'Uploaded' : 'Created', file.created_at ? new Date(file.created_at).toLocaleString() : '-']);
      rows.push(['Part count', file.type === 'file' ? formatPartCount(file.part_count) : '-']);
      if (file.type === 'file') {
        const activity = state.detailsDownloadActivity;
        const total = activity && Number.isInteger(activity.total) ? activity.total : 0;
        const authCount = activity && Number.isInteger(activity.auth) ? activity.auth : 0;
        const publicCount = activity && Number.isInteger(activity.public) ? activity.public : 0;
        rows.push(['Active downloads (now)', `${total} total (auth ${authCount}, public ${publicCount})`]);
      } else {
        rows.push(['Active downloads (now)', '-']);
      }
      if (state.user && file.owner_id === state.user.id && file.type === 'file') {
        const publicLinkCount = Number.isInteger(file.public_link_count) ? file.public_link_count : null;
        const passwordCount = Number.isInteger(file.public_link_password_count) ? file.public_link_password_count : null;
        rows.push(['Public links', publicLinkCount === null ? '-' : (publicLinkCount === 0 ? 'No public links' : `Public links: ${publicLinkCount}`)]);
        rows.push(['Password protected links', passwordCount === null ? '-' : (publicLinkCount === 0 ? '-' : `Password protected links: ${passwordCount}`)]);
      } else {
        rows.push(['Public links', '-']);
        rows.push(['Password protected links', '-']);
      }
      el.detailsBody.innerHTML = rows.map(([label, value]) => `
        <tr>
          <th>${escapeHTML(label)}</th>
          <td>${escapeHTML(value)}</td>
        </tr>
      `).join('');
    }

    async function revokePublicLink(linkID) {
      if (!requireWritableAction(setShareStatus)) return;
      if (!state.shareFile || !linkID) return;
      try {
        await api(`/files/${state.shareFile.id}/public-links/${linkID}`, { method: 'DELETE', csrf: true });
        setShareStatus('Public link revoked.');
        await refreshPublicLinks();
      } catch (err) {
        setShareStatus(err.message, true);
      }
    }

    async function copyPublicLink() {
      if (!el.publicLinkInput.value) return;
      try {
        await navigator.clipboard.writeText(el.publicLinkInput.value);
        setShareStatus('Public link copied.');
      } catch (_) {
        el.publicLinkInput.focus();
        el.publicLinkInput.select();
        setShareStatus('Select the link and copy it.');
      }
    }

    function selectedShareExpiry() {
      return selectedExpiry(el.shareExpirySelect.value);
    }

    function sharePermissionLabel(permission) {
      if (permission === 'read_delete') return 'Read + delete';
      return 'Read only';
    }

    function formatOwnerLabel(file) {
      if (state.user && file.owner_id === state.user.id) return 'You';
      const displayName = String(file.owner_display_name || '').trim();
      if (displayName) return displayName;
      const username = String(file.owner_username || '').trim();
      if (username) return `@${username}`;
      if (Number.isInteger(file.owner_telegram_id) && file.owner_telegram_id > 0) return `Telegram ${file.owner_telegram_id}`;
      return 'Unknown owner';
    }

    function formatAccessLabel(file, editable) {
      const access = String(file.access || '').trim();
      if (access === 'shared_read_delete') return 'Shared (read + delete)';
      if (access === 'shared_read') return 'Shared (read-only)';
      return editable ? 'Owner (read/write)' : 'Shared (read-only)';
    }

    function selectedExpiry(value) {
      if (!value) return undefined;
      const date = new Date();
      if (value === '1h') date.setHours(date.getHours() + 1);
      if (value === '1d') date.setDate(date.getDate() + 1);
      if (value === '7d') date.setDate(date.getDate() + 7);
      return date.toISOString();
    }

    function setShareStatus(message, error = false) {
      el.shareStatus.textContent = message;
      el.shareStatus.classList.toggle('error', error);
    }

    async function uploadSelected() {
      if (!requireWritableAction()) return;
      const files = selectedUploadFiles();
      if (!files.length) return;
      if (state.view !== 'own') {
        setUploadStatus('Switch to Own files to upload.', true);
        return;
      }
      const entries = normalizeUploadEntries(files, state.currentFolderId || '');
      const attached = attachFilesToPendingResumes(entries);
      if (attached.unmatched.length) enqueueFiles(attached.unmatched, state.currentFolderId || '');
      el.fileInput.value = '';
      state.droppedFiles = [];
      updateDropZone();
      const queued = attached.unmatched.length;
      const resumed = attached.resumed;
      if (queued && resumed) {
        setUploadStatus(`Queued ${queued} file${queued === 1 ? '' : 's'}, resumed ${resumed}.`);
      } else if (queued) {
        setUploadStatus(`Queued ${queued} file${queued === 1 ? '' : 's'}.`);
      } else {
        setUploadStatus(`Resumed ${resumed} file${resumed === 1 ? '' : 's'}.`);
      }
      runUploadQueue();
    }

    function selectedUploadFiles() {
      return state.droppedFiles.length ? state.droppedFiles : Array.from(el.fileInput.files || []);
    }

    function queueFileMeta(file) {
      if (!file) return { name: 'Upload', size: 0, type: '', lastModified: 0 };
      return {
        name: typeof file.name === 'string' ? file.name : 'Upload',
        size: Number.isFinite(Number(file.size)) ? Number(file.size) : 0,
        type: typeof file.type === 'string' ? file.type : '',
        lastModified: Number.isFinite(Number(file.lastModified)) ? Number(file.lastModified) : 0,
      };
    }

    function uploadFingerprint(file, parentID, displayPath) {
      const meta = queueFileMeta(file);
      return {
        name: meta.name,
        size: meta.size,
        lastModified: meta.lastModified,
        parentID: parentID || '',
        displayPath: displayPath || '',
      };
    }

    function sameUploadFingerprint(left, right) {
      if (!left || !right) return false;
      return (
        String(left.name || '') === String(right.name || '') &&
        Number(left.size || 0) === Number(right.size || 0) &&
        Number(left.lastModified || 0) === Number(right.lastModified || 0) &&
        String(left.parentID || '') === String(right.parentID || '') &&
        String(left.displayPath || '') === String(right.displayPath || '')
      );
    }

    function normalizeUploadEntries(files, parentID) {
      return files.map((entry) => {
        const file = entry && entry.file ? entry.file : entry;
        return {
          file,
          parentID: entry && typeof entry.parentID === 'string' ? entry.parentID : (parentID || ''),
          displayPath: entry && typeof entry.displayPath === 'string' ? entry.displayPath : '',
        };
      }).filter((entry) => entry.file);
    }

    function attachFilesToPendingResumes(entries) {
      let resumed = 0;
      const unmatched = [];
      for (const entry of entries) {
        const fingerprint = uploadFingerprint(entry.file, entry.parentID, entry.displayPath);
        const item = state.uploadQueue.find((candidate) => (
          candidate &&
          candidate.fileMissing &&
          candidate.status === 'needs_file' &&
          sameUploadFingerprint(candidate.resumeFingerprint, fingerprint)
        ));
        if (!item) {
          unmatched.push(entry);
          continue;
        }
        resumed += 1;
        updateQueueItem(item.id, {
          file: entry.file,
          parentID: entry.parentID || '',
          displayPath: entry.displayPath || '',
          fileMissing: false,
          safeToClose: false,
          progress: 0,
          status: 'queued',
          message: item.uploadID ? 'Resuming upload...' : 'Waiting',
          error: '',
          errorDetail: '',
          resumeFingerprint: fingerprint,
        });
      }
      return { resumed, unmatched };
    }

    function enqueueFiles(files, parentID) {
      const normalized = normalizeUploadEntries(files, parentID);
      for (const entry of normalized) {
        const meta = queueFileMeta(entry.file);
        const fingerprint = uploadFingerprint(entry.file, entry.parentID, entry.displayPath);
        state.uploadQueue.push({
          id: state.nextUploadQueueID,
          file: entry.file,
          parentID: entry.parentID || '',
          displayPath: entry.displayPath || '',
          status: 'queued',
          progress: 0,
          message: 'Waiting',
          error: '',
          safeToClose: false,
          cancelRequested: false,
          abortController: null,
          fileMissing: false,
          mimeType: meta.type || 'application/octet-stream',
          lastModified: meta.lastModified || 0,
          resumeFingerprint: fingerprint,
          createIdempotencyKey: newUploadIdempotencyKey(),
        });
        state.nextUploadQueueID += 1;
      }
      persistUploadQueueState();
      renderUploadQueue();
      if (state.view === 'own') {
        renderFiles(state.serverFiles || []);
      }
    }

    function shouldRerenderQueueRows(prev, next) {
      const prevVisible = queueItemVisibleInFileList(prev);
      const nextVisible = queueItemVisibleInFileList(next);
      if (prevVisible !== nextVisible) return true;
      const prevStatus = queueStatusForFileList(prev);
      const nextStatus = queueStatusForFileList(next);
      if (prevStatus !== nextStatus) return true;
      if (String(prev.uploadID || '') !== String(next.uploadID || '')) return true;
      return false;
    }

    function updateQueueItem(id, patch) {
      const index = state.uploadQueue.findIndex((item) => item.id === id);
      if (index < 0) return;
      const prev = state.uploadQueue[index];
      const next = { ...prev, ...patch };
      state.uploadQueue[index] = next;
      persistUploadQueueState();
      renderUploadQueue();
      if (state.view === 'own' && shouldRerenderQueueRows(prev, next)) {
        renderFiles(state.serverFiles || []);
      }
    }

    async function runUploadQueue() {
      if (state.readOnlyMapMode) {
        setUploadStatus(readOnlyMapModeMessage(), true);
        return;
      }
      if (state.uploadQueueRunning) return;
      state.uploadQueueRunning = true;
      try {
        while (true) {
          const next = state.uploadQueue.find((item) => item.status === 'queued' && !item.fileMissing);
          if (!next) break;
          try {
            await uploadQueueItem(next);
          } catch (err) {
            const uploadError = normalizeUploadError(err);
            debugUpload('upload:failed', {
              file: next.file.name,
              size: next.file.size,
              name: uploadError.name,
              message: uploadError.message,
              stack: uploadError.stack || '',
            });
            if (uploadError.message === 'upload_resume_file_required') {
              updateQueueItem(next.id, {
                status: 'needs_file',
                message: 'Resume available. Select the same file and click Upload.',
                error: '',
                errorDetail: '',
                safeToClose: true,
                fileMissing: true,
              });
              setUploadStatus(`Resume paused: ${next.file.name}`, true);
              continue;
            }
            updateQueueItem(next.id, {
              status: 'failed',
              message: uploadError.message,
              error: uploadError.message,
              errorDetail: uploadError.detail || '',
            });
            setUploadStatus(`Failed: ${next.file.name} (${uploadError.message})`, true);
          }
        }
      } finally {
        state.uploadQueueRunning = false;
      }
    }

    async function uploadQueueItem(item) {
      if (item.fileMissing) {
        throw new Error('upload_resume_file_required');
      }
      debugUpload('upload:start', {
        file: item.file.name,
        size: item.file.size,
        parent_id: item.parentID || null,
        mime_type: item.file.type || 'application/octet-stream',
      });
      if (item.cancelRequested) throw new Error('upload_canceled');
      let created;
      if (item.uploadID) {
        updateQueueItem(item.id, { status: 'staging', progress: 5, message: 'Resuming upload...' });
        const resumed = await api(`/uploads/${item.uploadID}`);
        created = { upload: resumed.upload };
      } else {
        updateQueueItem(item.id, { status: 'hashing', progress: 1, message: 'Hashing...' });
        const checksum = await sha256Hex(item.file, ({ loaded, total }) => {
          if (item.cancelRequested) throw new Error('upload_canceled');
          const percent = total > 0 ? Math.max(1, Math.round((loaded / total) * 4)) : 1;
          updateQueueItem(item.id, {
            status: 'hashing',
            progress: percent,
            message: `Hashing ${formatBytes(loaded)} / ${formatBytes(total)}`,
          });
        });
        if (item.cancelRequested) throw new Error('upload_canceled');
        debugUpload('upload:hash-complete', {
          file: item.file.name,
          checksum_prefix: checksum.slice(0, 12),
        });
        updateQueueItem(item.id, { status: 'staging', progress: 5, message: 'Creating upload...' });
        created = await api('/uploads', {
          method: 'POST',
          csrf: true,
          body: JSON.stringify({
            name: item.file.name,
            parent_id: item.parentID || undefined,
            mime_type: item.file.type || 'application/octet-stream',
            plaintext_size: item.file.size,
            checksum,
            idempotency_key: item.createIdempotencyKey || newUploadIdempotencyKey(),
          }),
        });
      }
      debugUpload('upload:created', {
        upload_id: created.upload.id,
        part_size: created.upload.part_size,
        part_count: created.upload.part_count,
        next_part_number: created.upload.next_part_number,
        plan: (created.upload.part_plan || []).map((part) => `${part.part_number}:${formatBytes(part.size || (part.end - part.start))}`).join(', '),
      });
      updateQueueItem(item.id, {
        uploadID: created.upload.id,
        uploadCreatedAt: created.upload.created_at || new Date().toISOString(),
        fileMissing: false,
      });
      if (state.view === 'own' && sameFolderID(item.parentID, state.currentFolderId || '')) {
        await refreshFiles();
      }
      await uploadParts(created.upload, item);
      if (item.cancelRequested) throw new Error('upload_canceled');
      updateQueueItem(item.id, {
        status: 'telegram',
        progress: 50,
        message: 'Server received. Tab can close.',
        safeToClose: true,
      });
      const readyState = await waitUntilReady(created.upload.id, item);
      if (item.cancelRequested) throw new Error('upload_canceled');
      if (readyState !== 'complete') {
        updateQueueItem(item.id, { status: 'completing', progress: 96, message: 'Completing...', safeToClose: true });
        debugUpload('upload:complete-request', { upload_id: created.upload.id });
        try {
          await api(`/uploads/${created.upload.id}/complete`, { method: 'POST', csrf: true });
        } catch (err) {
          if (err.message !== 'upload_not_found') throw err;
          debugUpload('upload:complete-race', { upload_id: created.upload.id, message: err.message });
        }
        debugUpload('upload:complete-ok', { upload_id: created.upload.id });
      }
      updateQueueItem(item.id, { status: 'complete', progress: 100, message: 'Complete', error: '' });
      setUploadStatus(`Uploaded ${item.file.name}`);
      await refreshFiles();
    }

    function updateDropZone() {
      const files = selectedUploadFiles();
      if (!files.length) {
        el.dropZone.textContent = 'Drop files or folders here or click to choose';
      } else if (files.length === 1) {
        el.dropZone.textContent = files[0].name;
      } else {
        el.dropZone.textContent = `${files.length} files selected`;
      }
    }

    function saveFolderState() {
      localStorage.setItem('tdv.folderState', JSON.stringify({
        currentFolderId: state.currentFolderId,
        folderStack: state.folderStack,
      }));
    }

    function restoreFolderState() {
      try {
        const saved = JSON.parse(localStorage.getItem('tdv.folderState') || '{}');
        state.currentFolderId = typeof saved.currentFolderId === 'string' ? saved.currentFolderId : '';
        state.folderStack = Array.isArray(saved.folderStack) ? saved.folderStack.filter((folder) => (
          folder && typeof folder.id === 'string' && typeof folder.name === 'string'
        )) : [];
      } catch (_) {
        state.currentFolderId = '';
        state.folderStack = [];
      }
    }

    function persistUploadQueueState() {
      const durable = state.uploadQueue.filter((item) => !['complete', 'failed'].includes(item.status)).map((item) => ({
        id: item.id,
        uploadID: item.uploadID || '',
        createIdempotencyKey: item.createIdempotencyKey || '',
        name: item.file && item.file.name ? item.file.name : 'Upload',
        size: item.file && Number.isFinite(Number(item.file.size)) ? Number(item.file.size) : 0,
        mimeType: item.mimeType || (item.file && item.file.type) || 'application/octet-stream',
        lastModified: Number.isFinite(Number(item.lastModified)) ? Number(item.lastModified) : Number(item.file && item.file.lastModified) || 0,
        parentID: item.parentID || '',
        displayPath: item.displayPath || '',
        status: item.status,
        progress: Number(item.progress) || 0,
        message: item.message || '',
        uploadCreatedAt: item.uploadCreatedAt || null,
        safeToClose: Boolean(item.safeToClose),
        fileMissing: Boolean(item.fileMissing),
        resumeFingerprint: item.resumeFingerprint || null,
      }));
      localStorage.setItem('tdv.uploadQueue', JSON.stringify(durable));
    }

    function restoreUploadQueueState() {
      try {
        const saved = JSON.parse(localStorage.getItem('tdv.uploadQueue') || '[]');
        if (!Array.isArray(saved)) return;
        const restored = saved.filter((item) => item).map((item) => {
          const safeToClose = Boolean(item.safeToClose);
          const hasUploadID = typeof item.uploadID === 'string' && item.uploadID;
          const fileMissing = !safeToClose;
          const displayPath = typeof item.displayPath === 'string' ? item.displayPath : '';
          const parentID = typeof item.parentID === 'string' ? item.parentID : '';
          const fingerprint = item.resumeFingerprint || {
            name: typeof item.name === 'string' ? item.name : 'Upload',
            size: Number(item.size) || 0,
            lastModified: Number(item.lastModified) || 0,
            parentID,
            displayPath,
          };
          return {
          id: Number(item.id) || state.nextUploadQueueID++,
          uploadID: hasUploadID ? item.uploadID : '',
          createIdempotencyKey: typeof item.createIdempotencyKey === 'string' ? item.createIdempotencyKey : newUploadIdempotencyKey(),
          uploadCreatedAt: item.uploadCreatedAt || null,
          file: {
            name: typeof item.name === 'string' && item.name ? item.name : 'Upload',
            size: Number(item.size) || 0,
            type: typeof item.mimeType === 'string' ? item.mimeType : 'application/octet-stream',
            lastModified: Number(item.lastModified) || 0,
          },
          mimeType: typeof item.mimeType === 'string' ? item.mimeType : 'application/octet-stream',
          lastModified: Number(item.lastModified) || 0,
          parentID,
          displayPath,
          status: safeToClose ? (item.status === 'completing' ? 'completing' : 'telegram') : 'needs_file',
          progress: safeToClose ? (Number(item.progress) || 50) : 0,
          message: safeToClose
            ? (typeof item.message === 'string' && item.message ? item.message : 'Restored. Server/worker can continue.')
            : 'Resume available. Select the same file and click Upload.',
          error: '',
          safeToClose: safeToClose || fileMissing,
          fileMissing,
          cancelRequested: false,
          abortController: null,
          resumeFingerprint: fingerprint,
        };
        });
        state.uploadQueue = restored;
        const maxID = restored.reduce((max, item) => Math.max(max, item.id), 0);
        state.nextUploadQueueID = Math.max(state.nextUploadQueueID, maxID + 1);
        renderUploadQueue();
      } catch (_) {
        localStorage.removeItem('tdv.uploadQueue');
      }
    }

    async function monitorRestoredUploads() {
      if (state.uploadMonitorRunning) return;
      state.uploadMonitorRunning = true;
      try {
        while (true) {
          const item = state.uploadQueue.find((entry) => (
            entry.uploadID &&
            entry.safeToClose &&
            (entry.status === 'telegram' || entry.status === 'completing')
          ));
          if (!item) break;
          try {
            await monitorServerUpload(item);
          } catch (err) {
            updateQueueItem(item.id, {
              status: 'failed',
              message: err.message || 'Upload status check failed',
              error: err.message || 'Upload status check failed',
            });
          }
        }
      } finally {
        state.uploadMonitorRunning = false;
      }
    }

    async function monitorServerUpload(item) {
      const readyState = await waitUntilReady(item.uploadID, item);
      if (item.cancelRequested) throw new Error('upload_canceled');
      if (readyState !== 'complete') {
        updateQueueItem(item.id, { status: 'completing', progress: 96, message: 'Completing...', safeToClose: true });
        try {
          await api(`/uploads/${item.uploadID}/complete`, { method: 'POST', csrf: true });
        } catch (err) {
          if (err.message !== 'upload_not_found') throw err;
        }
      }
      updateQueueItem(item.id, { status: 'complete', progress: 100, message: 'Complete', error: '' });
      setUploadStatus(`Uploaded ${item.file.name}`);
      await refreshFiles();
    }

    async function uploadParts(upload, item) {
      const plan = uploadPartPlan(upload, item.file.size);
      const count = plan.length;
      const startPartNumber = uploadStartPartNumber(upload, count);
      for (const partInfo of plan) {
        if (partInfo.partNumber < startPartNumber) {
          continue;
        }
        if (item.cancelRequested) throw new Error('upload_canceled');
        const part = partInfo.partNumber;
        const start = partInfo.start;
        const end = partInfo.end;
        updateQueueItem(item.id, {
          status: 'staging',
          message: `Staging part ${part}/${count}`,
          progress: 5 + Math.round((part / Math.max(count, 1)) * 45),
        });
        debugUpload('part:send', {
          upload_id: upload.id,
          part,
          count,
          start,
          end,
          size: end - start,
        });
        const abortHandle = { abort: null };
        updateQueueItem(item.id, { abortController: abortHandle });
        const payload = buildUploadBody(item.file, start, end);
        const transportChunkSize = browserTransportChunkSize();
        const useChunkedTransport = (end - start) > transportChunkSize;
        if (useChunkedTransport) payload.mode = `chunked-${formatBytes(transportChunkSize)}`;
        debugUpload('part:send-mode', {
          upload_id: upload.id,
          part,
          mode: payload.mode,
          size: end - start,
        });
        let response;
        let lastProgress = { loaded: 0, total: end - start };
        let lastProgressBucket = -10;
        const startedAt = Date.now();
        const requestContext = {
          upload_id: upload.id,
          file_name: item.file.name,
          part_number: part,
          part_count: count,
          part_size: end - start,
          mode: payload.mode,
          transport: 'xhr',
        };
        const onProgress = ({ loaded, total, chunkIndex, chunkCount }) => {
          lastProgress = { loaded, total };
          if (!total || total <= 0) return;
          const partPercent = Math.max(0, Math.min(100, Math.round((loaded / total) * 100)));
          if (partPercent >= lastProgressBucket + 10 || partPercent === 100) {
            lastProgressBucket = partPercent;
            debugUpload('part:progress', {
              upload_id: upload.id,
              part,
              percent: partPercent,
              loaded,
              total,
              chunk: chunkIndex || null,
              chunks: chunkCount || null,
            });
          }
          const partBase = (part - 1) / Math.max(count, 1);
          const overall = 5 + Math.round(((partBase + (partPercent / 100) / Math.max(count, 1)) * 45));
          const chunkSuffix = chunkIndex && chunkCount ? ` chunk ${chunkIndex}/${chunkCount}` : '';
          updateQueueItem(item.id, {
            status: 'staging',
            progress: overall,
            message: `Staging part ${part}/${count}${chunkSuffix} (${partPercent}%)`,
          });
        };
        let attempt = 1;
        while (true) {
          try {
            if (useChunkedTransport) {
              response = await uploadPartChunked(upload.id, part, item.file, start, end, {
                csrf: csrfToken(),
                abortHandle,
                chunkSize: transportChunkSize,
                onProgress,
              });
            } else {
              response = await uploadPartXHR(`/uploads/${upload.id}/parts/${part}`, payload.body, {
                csrf: csrfToken(),
                abortHandle,
                onProgress,
              });
            }
          } catch (err) {
            const networkError = normalizeUploadError(err);
            if (isRetryableUploadTransportError(err) && attempt < uploadPartMaxAttempts()) {
              const delayMs = uploadPartRetryDelayMS(attempt);
              debugUpload('part:retry-transport', {
                upload_id: upload.id,
                part,
                attempt,
                next_attempt: attempt + 1,
                retry_ms: delayMs,
                message: networkError.message,
              });
              updateQueueItem(item.id, {
                status: 'staging',
                message: `Retrying part ${part}/${count} after transport error (attempt ${attempt + 1}/${uploadPartMaxAttempts()})`,
              });
              await delay(delayMs);
              attempt += 1;
              continue;
            }
            updateQueueItem(item.id, { abortController: null });
            networkError.detail = uploadErrorDetail({
              ...requestContext,
              status: err && Number.isFinite(Number(err.status)) ? Number(err.status) : 0,
              status_text: err && err.statusText ? err.statusText : '',
              error_code: err && err.code ? err.code : networkError.message,
              message: err && err.message ? err.message : networkError.message,
              loaded: lastProgress.loaded || 0,
              total: lastProgress.total || end - start,
              elapsed_ms: Date.now() - startedAt,
            });
            debugUpload('part:xhr-error', {
              upload_id: upload.id,
              part,
              count,
              size: end - start,
              status: err && err.status ? err.status : 0,
              status_text: err && err.statusText ? err.statusText : '',
              event: err && err.event ? err.event : '',
              name: networkError.name,
              message: networkError.message,
              detail: networkError.detail,
              stack: networkError.stack || '',
            });
            await reportUploadClientEvent({
              event: 'upload_part_transport_failed',
              ...requestContext,
              status: err && Number.isFinite(Number(err.status)) ? Number(err.status) : 0,
              status_text: err && err.statusText ? err.statusText : '',
              error_code: err && err.code ? err.code : networkError.message,
              message: networkError.message,
              loaded: lastProgress.loaded || 0,
              total: lastProgress.total || end - start,
              elapsed_ms: Date.now() - startedAt,
            });
            const recovered = await recoverUploadPartProgress(upload.id, part);
            if (recovered.accepted) {
              debugUpload('part:recovered-after-transport-failure', {
                upload_id: upload.id,
                part,
                next_part_number: recovered.nextPartNumber || null,
                upload_status: recovered.status || null,
              });
              break;
            }
            throw networkError;
          }
          updateQueueItem(item.id, { abortController: null });
          debugUpload('part:response', {
            upload_id: upload.id,
            part,
            status: response.status,
            ok: response.ok,
            attempt,
          });
          if (!response.ok && response.status !== 202) {
            const body = response.body || {};
            const code = body.error || 'upload_part_http_failed';
            const message = uploadHTTPErrorMessage(response.status, response.statusText, code);
            if (response.status === 409 && code === 'upload_part_out_of_order') {
              const recovered = await recoverUploadPartProgress(upload.id, part);
              if (recovered.accepted) {
                debugUpload('part:recovered-after-out-of-order', {
                  upload_id: upload.id,
                  part,
                  next_part_number: recovered.nextPartNumber || null,
                  upload_status: recovered.status || null,
                });
                break;
              }
            }
            if (isRetryableUploadHTTPStatus(response.status) && attempt < uploadPartMaxAttempts()) {
              const delayMs = uploadPartRetryDelayMS(attempt);
              debugUpload('part:retry-http', {
                upload_id: upload.id,
                part,
                attempt,
                next_attempt: attempt + 1,
                retry_ms: delayMs,
                status: response.status,
                error_code: code,
              });
              updateQueueItem(item.id, {
                status: 'staging',
                message: `Retrying part ${part}/${count} after server error ${response.status} (attempt ${attempt + 1}/${uploadPartMaxAttempts()})`,
              });
              await delay(delayMs);
              attempt += 1;
              continue;
            }
            const err = new Error(message);
            err.code = code;
            err.status = response.status;
            err.statusText = response.statusText || '';
            err.detail = uploadErrorDetail({
              ...requestContext,
              status: response.status,
              status_text: response.statusText || '',
              error_code: code,
              message,
              loaded: lastProgress.loaded || 0,
              total: lastProgress.total || end - start,
              elapsed_ms: Date.now() - startedAt,
            });
            debugUpload('part:error', { upload_id: upload.id, part, status: response.status, message, detail: err.detail });
            await reportUploadClientEvent({
              event: 'upload_part_http_failed',
              ...requestContext,
              status: response.status,
              status_text: response.statusText || '',
              error_code: code,
              message,
              loaded: lastProgress.loaded || 0,
              total: lastProgress.total || end - start,
              elapsed_ms: Date.now() - startedAt,
            });
            throw err;
          }
          break;
        }
      }
    }

    function uploadStartPartNumber(upload, count) {
      const nextPartNumber = Number(upload && upload.next_part_number);
      if (!Number.isInteger(nextPartNumber) || nextPartNumber < 1) return 1;
      if (nextPartNumber > count + 1) return count + 1;
      return nextPartNumber;
    }

    function uploadPartMaxAttempts() {
      return 3;
    }

    function uploadPartRetryDelayMS(attempt) {
      const current = Math.max(1, Number(attempt) || 1);
      return Math.min(5000, 600 * current);
    }

    function isRetryableUploadHTTPStatus(status) {
      return [500, 502, 503, 504].includes(Number(status));
    }

    function isRetryableUploadTransportError(err) {
      if (!err) return false;
      const code = String(err.code || '').trim();
      return (
        code === 'upload_part_network_failed' ||
        code === 'upload_part_timeout' ||
        code === 'network_upload_failed'
      );
    }

    async function recoverUploadPartProgress(uploadID, partNumber) {
      try {
        const data = await api(`/uploads/${uploadID}`);
        const nextPartNumber = Number(data && data.upload ? data.upload.next_part_number : 0);
        const uploadStatus = String(data && data.upload && data.upload.status ? data.upload.status : '');
        const acceptedByCursor = Number.isInteger(nextPartNumber) && nextPartNumber > partNumber;
        const acceptedByStatus = uploadStatus === 'complete';
        if (acceptedByCursor || acceptedByStatus) {
          return { accepted: true, nextPartNumber, status: uploadStatus };
        }
      } catch (err) {
        debugUpload('part:recover-check-failed', {
          upload_id: uploadID,
          part: partNumber,
          message: err && err.message ? err.message : 'recover check failed',
        });
      }
      return { accepted: false };
    }

    function newUploadIdempotencyKey() {
      const prefix = 'upload-';
      const randomLength = 16;
      if (window.crypto && window.crypto.getRandomValues) {
        const bytes = new Uint8Array(randomLength);
        window.crypto.getRandomValues(bytes);
        return `${prefix}${Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')}`;
      }
      return `${prefix}${Date.now().toString(16)}${Math.floor(Math.random() * 0xFFFFFFFF).toString(16).padStart(8, '0')}`;
    }

    function uploadPartPlan(upload, fileSize) {
      const serverPlan = Array.isArray(upload.part_plan) ? upload.part_plan : [];
      const normalized = serverPlan.map((part) => ({
        partNumber: Number(part.part_number),
        start: Number(part.start),
        end: Number(part.end),
      })).filter((part) => (
        Number.isInteger(part.partNumber) &&
        Number.isFinite(part.start) &&
        Number.isFinite(part.end) &&
        part.partNumber > 0 &&
        part.start >= 0 &&
        part.end >= part.start &&
        part.end <= fileSize
      ));
      if (normalized.length) {
        return normalized.sort((a, b) => a.partNumber - b.partNumber);
      }

      const partSize = Number(upload.part_size);
      const count = Number(upload.part_count);
      if (!Number.isFinite(partSize) || partSize <= 0 || !Number.isInteger(count) || count < 1) {
        throw new Error('Upload part plan is invalid.');
      }
      const fallback = [];
      for (let part = 1; part <= count; part++) {
        const start = (part - 1) * partSize;
        fallback.push({
          partNumber: part,
          start,
          end: Math.min(start + partSize, fileSize),
        });
      }
      return fallback;
    }

    async function waitUntilReady(uploadID, item) {
      clearInterval(state.uploadTimer);
      let pollErrors = 0;
      while (true) {
        if (item.cancelRequested) throw new Error('upload_canceled');
        let data;
        try {
          data = await api(`/uploads/${uploadID}`);
          pollErrors = 0;
        } catch (err) {
          pollErrors += 1;
          debugUpload('upload:poll-error', {
            upload_id: uploadID,
            attempts: pollErrors,
            message: err.message || 'poll failed',
          });
          updateQueueItem(item.id, {
            status: 'telegram',
            message: `Waiting for API reconnect (${pollErrors})...`,
            safeToClose: true,
          });
          await delay(Math.min(10000, 2000 * pollErrors));
          continue;
        }
        const progress = data.progress;
        if (data.upload && data.upload.status === 'complete') return 'complete';
        if (data.upload && (data.upload.status === 'failed' || data.upload.status === 'expired')) {
          throw new Error(`upload_${data.upload.status}`);
        }
        const done = progress.complete_parts;
        const total = progress.expected_parts;
        const queued = progress.queued_parts;
        const leased = progress.leased_parts;
        debugUpload('upload:poll', {
          upload_id: uploadID,
          done,
          total,
          queued,
          leased,
          failed: progress.failed_parts,
          next_retry_at: progress.next_retry_at || null,
          eta_source: progress.telegram_eta_source || null,
          estimated_bps: progress.telegram_estimated_bps || 0,
        });
        if (progress.failed_parts > 0) throw new Error('upload_part_failed');
        if (progress.ready_to_complete) return 'ready';
        const eta = formatETA(progress.telegram_eta_seconds);
        const speed = formatSpeed(progress.telegram_estimated_bps);
        const retry = formatRetryAt(progress.next_retry_at);
        const retrySuffix = retry ? `, retry ${retry}` : '';
        updateQueueItem(item.id, {
          status: 'telegram',
          message: `Telegram ${done}/${total} complete, ${leased} active, ${queued} queued, ETA ${eta}, ${speed}${retrySuffix}`,
          safeToClose: true,
          progress: 50 + Math.round((done / Math.max(total, 1)) * 45),
        });
        await delay(2000);
      }
    }

    function queueHasCompletedItems() {
      return state.uploadQueue.some((item) => item.status === 'complete');
    }

    function queueItemActions(item) {
      if (item.status === 'needs_file') {
        return `
          <div class="queue-actions">
            <button data-queue-remove="${item.id}">Remove</button>
          </div>
        `;
      }
      if (item.status === 'failed') {
        return `
          <div class="queue-actions">
            <button data-queue-retry="${item.id}">Retry</button>
            <button data-queue-remove="${item.id}">Remove</button>
          </div>
        `;
      }
      if (item.status === 'complete') {
        return `
          <div class="queue-actions">
            <button data-queue-remove="${item.id}">Remove</button>
          </div>
        `;
      }
      return `
        <div class="queue-actions">
          <button data-queue-cancel="${item.id}">Cancel</button>
        </div>
      `;
    }

    function renderUploadQueue() {
      if (!state.uploadQueue.length) {
        el.uploadQueue.innerHTML = '<div class="queue-empty">Queue is empty.</div>';
        el.clearCompletedQueueBtn.classList.add('hidden');
        updateTabSafetyIndicator();
        return;
      }
      el.clearCompletedQueueBtn.classList.toggle('hidden', !queueHasCompletedItems());
      el.uploadQueue.innerHTML = state.uploadQueue.map((item) => {
        const statusClass = item.status === 'failed' ? 'error' : (item.status === 'complete' ? 'ok' : 'warn');
        const label = item.status === 'failed' ? (item.error || item.message || 'Failed') : item.message;
        const detail = item.status === 'failed' && item.errorDetail
          ? `<div class="queue-detail">${escapeHTML(item.errorDetail)}</div>`
          : '';
        return `
          <div class="queue-item">
            <div class="queue-head">
              <div class="queue-name">${escapeHTML(item.file.name)}</div>
              <div class="queue-meta">${formatBytes(item.file.size || 0)}</div>
            </div>
            <div class="queue-status"><span class="badge ${statusClass}">${escapeHTML(item.status)}</span> ${escapeHTML(label || '')}</div>
            ${detail}
            <progress value="${Math.max(0, Math.min(100, Number(item.progress || 0)))}" max="100"></progress>
            ${queueItemActions(item)}
          </div>
        `;
      }).join('');
      wireUploadQueueActions();
      updateTabSafetyIndicator();
    }

    function updateTabSafetyIndicator() {
      const unsafe = state.uploadQueue.some((item) => !item.safeToClose && !['complete', 'failed'].includes(item.status));
      el.tabSafetyIndicator.classList.toggle('unsafe', unsafe);
      el.tabSafetyText.textContent = unsafe
        ? 'Keep this tab open until server receive finishes.'
        : 'Tab can close. Server/worker can continue.';
    }

    function wireUploadQueueActions() {
      el.uploadQueue.querySelectorAll('[data-queue-retry]').forEach((button) => {
        button.addEventListener('click', () => retryQueueItem(button.dataset.queueRetry));
      });
      el.uploadQueue.querySelectorAll('[data-queue-remove]').forEach((button) => {
        button.addEventListener('click', () => removeQueueItem(button.dataset.queueRemove));
      });
      el.uploadQueue.querySelectorAll('[data-queue-cancel]').forEach((button) => {
        button.addEventListener('click', () => cancelQueueItem(button.dataset.queueCancel));
      });
    }

    function retryQueueItem(id) {
      const item = state.uploadQueue.find((entry) => String(entry.id) === String(id));
      if (!item || item.status !== 'failed') return;
      const resumeNeedsFile = Boolean(item.fileMissing);
      updateQueueItem(item.id, {
        status: resumeNeedsFile ? 'needs_file' : 'queued',
        progress: 0,
        message: resumeNeedsFile ? 'Resume available. Select the same file and click Upload.' : 'Waiting',
        error: '',
        errorDetail: '',
        safeToClose: resumeNeedsFile,
        cancelRequested: false,
      });
      if (resumeNeedsFile) {
        setUploadStatus(`Select the same file to resume: ${item.file.name}`, true);
      } else {
        setUploadStatus(`Retry queued: ${item.file.name}`);
      }
      runUploadQueue();
    }

    function removeQueueItem(id) {
      const item = state.uploadQueue.find((entry) => String(entry.id) === String(id));
      if (!item || !['complete', 'failed', 'needs_file'].includes(item.status)) return;
      state.uploadQueue = state.uploadQueue.filter((entry) => String(entry.id) !== String(id));
      persistUploadQueueState();
      renderUploadQueue();
    }

    async function cancelQueueItem(id) {
      const item = state.uploadQueue.find((entry) => String(entry.id) === String(id));
      if (!item || item.status === 'complete' || item.status === 'failed') return;
      item.cancelRequested = true;
      if (item.abortController) item.abortController.abort();
      updateQueueItem(item.id, {
        status: 'failed',
        message: 'Canceled',
        error: 'Canceled',
        abortController: null,
      });
      if (item.uploadID) {
        try {
          await api(`/uploads/${item.uploadID}`, { method: 'DELETE', csrf: true });
        } catch (err) {
          debugUpload('upload:cancel-failed', { upload_id: item.uploadID, message: err.message });
        }
      }
      setUploadStatus(`Canceled: ${item.file.name}`);
      runUploadQueue();
    }

    function clearCompletedQueueItems() {
      state.uploadQueue = state.uploadQueue.filter((item) => item.status !== 'complete');
      persistUploadQueueState();
      renderUploadQueue();
    }

    function setUploadStatus(message, error = false) {
      el.uploadStatus.textContent = message;
      el.uploadStatus.classList.toggle('error', error);
    }

    function setUploadDebugEnabled(enabled) {
      state.uploadDebugEnabled = state.uploadDebugAllowed && enabled;
      localStorage.setItem('tdv.uploadDebug', state.uploadDebugEnabled ? '1' : '0');
      el.uploadDebugToggle.checked = state.uploadDebugEnabled;
      el.uploadDebugToggle.closest('label').classList.toggle('hidden', !state.uploadDebugAllowed);
      el.uploadDebugLog.classList.toggle('hidden', !state.uploadDebugEnabled);
      renderUploadDebugLog();
      if (state.uploadDebugEnabled) debugUpload('debug:enabled', { source: 'APP_DEBUG' });
    }

    function debugUpload(eventName, details = {}) {
      if (!state.uploadDebugEnabled) return;
      const line = {
        time: new Date().toISOString(),
        event: eventName,
        ...details,
      };
      state.uploadDebugLines.unshift(line);
      state.uploadDebugLines = state.uploadDebugLines.slice(0, 80);
      console.debug('[TeleVault upload]', line);
      renderUploadDebugLog();
    }

    async function reportUploadClientEvent(details) {
      try {
        debugUpload('client-event:send', details);
        await api('/uploads/client-events', {
          method: 'POST',
          csrf: true,
          body: JSON.stringify(details),
        });
      } catch (err) {
        debugUpload('client-event:failed', {
          upload_id: details && details.upload_id ? details.upload_id : '',
          message: err && err.message ? err.message : 'client event failed',
        });
      }
    }

    function uploadErrorDetail(details) {
      const parts = [];
      if (details.upload_id) parts.push(`upload=${details.upload_id}`);
      if (details.part_number) parts.push(`part=${details.part_number}/${details.part_count || '?'}`);
      if (details.part_size) parts.push(`size=${formatBytes(details.part_size)}`);
      if (details.status) parts.push(`status=${details.status}${details.status_text ? ` ${details.status_text}` : ''}`);
      if (details.error_code) parts.push(`code=${details.error_code}`);
      if (details.loaded || details.total) parts.push(`sent=${formatBytes(details.loaded || 0)}/${formatBytes(details.total || 0)}`);
      if (details.elapsed_ms) parts.push(`elapsed=${Math.round(details.elapsed_ms / 1000)}s`);
      return parts.join(' | ');
    }

    function renderUploadDebugLog() {
      if (!el.uploadDebugLog) return;
      if (!state.uploadDebugLines.length) {
        el.uploadDebugLog.innerHTML = '<button type="button" data-debug-clear>Clear</button><div class="debug-line">No upload debug events yet.</div>';
      } else {
        const lines = state.uploadDebugLines.map((line) => `<div class="debug-line">${escapeHTML(JSON.stringify(line))}</div>`).join('');
        el.uploadDebugLog.innerHTML = `<button type="button" data-debug-clear>Clear</button>${lines}`;
      }
      const clear = el.uploadDebugLog.querySelector('[data-debug-clear]');
      if (clear) {
        clear.addEventListener('click', () => {
          state.uploadDebugLines = [];
          renderUploadDebugLog();
        });
      }
    }

    function setRecoveryStatus(message, error = false) {
      el.recoveryStatus.textContent = message;
      el.recoveryStatus.classList.toggle('error', error);
    }

    async function exportRecovery() {
      el.exportRecoveryBtn.disabled = true;
      setRecoveryStatus('Exporting...');
      try {
        const response = await fetch('/recovery/export', {
          method: 'POST',
          headers: { 'X-CSRF-Token': csrfToken() },
          credentials: 'same-origin',
        });
        if (!response.ok) {
          let message = response.statusText;
          try {
            const body = await response.json();
            message = body.error || message;
          } catch (_) {}
          throw new Error(message);
        }
        const blob = await response.blob();
        const link = document.createElement('a');
        link.href = URL.createObjectURL(blob);
        link.download = `televault-recovery-${new Date().toISOString().replace(/[:.]/g, '-')}.json`;
        document.body.appendChild(link);
        link.click();
        link.remove();
        URL.revokeObjectURL(link.href);
        setRecoveryStatus('Recovery map exported.');
      } catch (err) {
        setRecoveryStatus(err.message, true);
      } finally {
        el.exportRecoveryBtn.disabled = false;
      }
    }

    function startRecoveryImport() {
      el.recoveryFileInput.value = '';
      el.recoveryFileInput.click();
    }

    async function importRecoveryFile() {
      const file = el.recoveryFileInput.files[0];
      if (!file) return;
      const confirmed = window.confirm('This import replaces your current vault metadata with the selected recovery map. Continue?');
      if (!confirmed) {
        setRecoveryStatus('Recovery import canceled.');
        return;
      }
      el.importRecoveryBtn.disabled = true;
      setRecoveryStatus('Importing...');
      try {
        const manifestText = await file.text();
        JSON.parse(manifestText);
        const data = await api('/recovery/import?mode=replace&confirm_replace=1', {
          method: 'POST',
          csrf: true,
          headers: { 'Content-Type': 'application/json' },
          body: manifestText,
        });
        const summary = data.import;
        const keyMode = summary && summary.used_existing_recovery_key
          ? 'Used existing recovery key.'
          : 'Imported recovery private key from map.';
        setRecoveryStatus(`Imported ${summary.files_imported} files and ${summary.parts_imported} parts. ${keyMode}`);
        await refreshFiles();
      } catch (err) {
        setRecoveryStatus(err.message, true);
      } finally {
        el.importRecoveryBtn.disabled = false;
      }
    }

    function setMFAStatus(message, error = false) {
      el.mfaStatus.textContent = message;
      el.mfaStatus.classList.toggle('error', error);
    }

    function clearSecurityDraft() {
      el.mfaTotpEnrollBox.classList.add('hidden');
      el.mfaTotpSecretInput.value = '';
      el.mfaTotpCodeInput.value = '';
      el.mfaTotpQR.classList.add('hidden');
      el.mfaTotpQR.src = '';
      el.mfaPasskeyNameInput.value = '';
      el.mfaLocalPasswordInput.value = '';
      el.mfaLocalPasswordConfirmInput.value = '';
      el.mfaLocalPasswordConfirmInput.classList.add('hidden');
      el.setLocalPasswordBtn.disabled = true;
    }

    function updateLocalPasswordActions() {
      const password = String(el.mfaLocalPasswordInput.value || '');
      const confirm = String(el.mfaLocalPasswordConfirmInput.value || '');
      const hasPassword = password.trim().length > 0;
      el.mfaLocalPasswordConfirmInput.classList.toggle('hidden', !hasPassword);
      if (!hasPassword) {
        el.setLocalPasswordBtn.disabled = true;
        return;
      }
      const validLength = password.trim().length >= 5;
      const matches = password === confirm && confirm.length > 0;
      el.setLocalPasswordBtn.disabled = !(validLength && matches);
    }

    function showRecoveryCodes(codes, prefix) {
      if (!Array.isArray(codes) || !codes.length) return;
      el.mfaRecoveryCodesOutput.value = codes.join('\n');
      el.mfaRecoveryCodesBox.classList.remove('hidden');
      setMFAStatus(prefix || 'Recovery codes generated. Save them now.');
    }

    function renderPasskeyRows(passkeys) {
      const items = Array.isArray(passkeys) ? passkeys : [];
      if (!items.length) {
        el.mfaPasskeysBody.innerHTML = '<tr><td class="muted">No passkeys configured.</td><td></td></tr>';
        return;
      }
      const rows = items.map((item) => {
        const id = String(item && item.id ? item.id : '');
        const displayName = String(item && item.display_name ? item.display_name : 'Passkey');
        const escapedName = escapeHTML(displayName);
        const escapedID = escapeHTML(id);
        return `<tr data-passkey-id="${escapedID}">
          <td>
            <input type="text" value="${escapedName}" maxlength="80" data-passkey-name="${escapedID}" aria-label="Passkey name">
          </td>
          <td class="row-actions">
            <button data-passkey-rename="${escapedID}">Save</button>
            <button data-passkey-delete="${escapedID}">Delete</button>
          </td>
        </tr>`;
      });
      el.mfaPasskeysBody.innerHTML = rows.join('');
    }

    function renderMFAStatus(status) {
      state.mfaStatus = status || null;
      const data = status || {};
      const totpEnabled = Boolean(data.totp_enabled);
      const webauthnCredentials = Array.isArray(data.webauthn_credentials) ? data.webauthn_credentials : [];
      const passwordConfigured = Boolean(data.password_configured);
      const remaining = Number.isFinite(Number(data.recovery_codes_remaining))
        ? Number(data.recovery_codes_remaining)
        : 0;

      el.mfaRecoveryRemainingInput.value = String(remaining);
      renderPasskeyRows(webauthnCredentials);
      if (passwordConfigured) {
        el.mfaLocalPasswordInput.placeholder = 'Configured. Enter new value to rotate.';
      } else {
        el.mfaLocalPasswordInput.placeholder = 'At least 5 characters';
      }
      el.startTotpEnrollBtn.textContent = totpEnabled ? 'Reconfigure TOTP' : 'Set up TOTP';
      el.regenerateRecoveryBtn.disabled = !totpEnabled;
      el.disableTotpBtn.disabled = !totpEnabled;
      el.disableLocalPasswordBtn.disabled = !passwordConfigured;
      updateLocalPasswordActions();
    }

    async function loadMFAStatus() {
      const data = await api('/auth/mfa/status');
      renderMFAStatus(data);
      return data;
    }

    async function openSecurityDialog() {
      if (!state.user) return;
      clearSecurityDraft();
      el.mfaRecoveryCodesBox.classList.add('hidden');
      el.mfaRecoveryCodesOutput.value = '';
      el.securityModal.classList.remove('hidden');
      setMFAStatus('Loading...');
      try {
        await loadMFAStatus();
        setMFAStatus('Loaded.');
      } catch (err) {
        setMFAStatus(err.message, true);
      }
    }

    function closeSecurityDialog() {
      el.securityModal.classList.add('hidden');
      clearSecurityDraft();
      el.mfaRecoveryCodesBox.classList.add('hidden');
      el.mfaRecoveryCodesOutput.value = '';
      setMFAStatus('');
    }

    async function startTotpEnrollmentFromSecurity() {
      if (!state.user) return;
      el.startTotpEnrollBtn.disabled = true;
      setMFAStatus('Starting TOTP setup...');
      try {
        const data = await api('/auth/mfa/totp/enroll/start', { method: 'POST', csrf: true });
        clearSecurityDraft();
        const totp = data && data.totp ? data.totp : {};
        el.mfaTotpSecretInput.value = totp.secret || '';
        if (totp.qr_image_url) {
          el.mfaTotpQR.src = totp.qr_image_url;
          el.mfaTotpQR.classList.remove('hidden');
        }
        el.mfaTotpEnrollBox.classList.remove('hidden');
        el.mfaTotpCodeInput.focus();
        setMFAStatus('Scan QR code and enter authenticator code.');
      } catch (err) {
        setMFAStatus(err.message, true);
      } finally {
        el.startTotpEnrollBtn.disabled = false;
      }
    }

    async function confirmTotpEnrollmentFromSecurity() {
      if (!state.user) return;
      const code = String(el.mfaTotpCodeInput.value || '').trim();
      if (!code) {
        setMFAStatus('Enter local 2FA code.', true);
        el.mfaTotpCodeInput.focus();
        return;
      }
      el.confirmTotpEnrollBtn.disabled = true;
      setMFAStatus('Confirming TOTP...');
      try {
        const data = await api('/auth/mfa/totp/enroll/confirm', {
          method: 'POST',
          csrf: true,
          body: JSON.stringify({ code }),
        });
        clearSecurityDraft();
        showRecoveryCodes(data.recovery_codes, 'TOTP enabled. Save recovery codes now.');
        await loadMFAStatus();
      } catch (err) {
        setMFAStatus(err.message, true);
      } finally {
        el.confirmTotpEnrollBtn.disabled = false;
      }
    }

    async function registerWebauthnFromSecurity() {
      if (!state.user) return;
      const displayName = String(el.mfaPasskeyNameInput.value || '').trim() || 'Passkey';
      el.registerWebauthnBtn.disabled = true;
      setMFAStatus('Waiting for WebAuthn device...');
      try {
        await runWebAuthnLocalMFARegister(displayName);
        el.mfaPasskeyNameInput.value = '';
        await loadMFAStatus();
        setMFAStatus('Passkey added.');
      } catch (err) {
        setMFAStatus(err.message, true);
      } finally {
        el.registerWebauthnBtn.disabled = false;
      }
    }

    async function regenerateRecoveryCodes() {
      if (!state.user) return;
      const confirmed = window.confirm('Regenerate recovery codes? Existing unused codes will stop working.');
      if (!confirmed) return;
      el.regenerateRecoveryBtn.disabled = true;
      setMFAStatus('Regenerating recovery codes...');
      try {
        const data = await api('/auth/mfa/recovery/regenerate', {
          method: 'POST',
          csrf: true,
        });
        showRecoveryCodes(data.recovery_codes, 'Recovery codes regenerated. Save them now.');
        await loadMFAStatus();
      } catch (err) {
        setMFAStatus(err.message, true);
      } finally {
        el.regenerateRecoveryBtn.disabled = false;
      }
    }

    async function setLocalPasswordFromSecurity() {
      if (!state.user) return;
      const password = String(el.mfaLocalPasswordInput.value || '');
      const confirm = String(el.mfaLocalPasswordConfirmInput.value || '');
      if (!password.trim()) {
        setMFAStatus('Enter local password.', true);
        el.mfaLocalPasswordInput.focus();
        return;
      }
      if (password.trim().length < 5) {
        setMFAStatus(humanizeError('local_password_too_short'), true);
        el.mfaLocalPasswordInput.focus();
        return;
      }
      if (password !== confirm) {
        setMFAStatus(humanizeError('local_password_mismatch'), true);
        el.mfaLocalPasswordConfirmInput.focus();
        return;
      }
      el.setLocalPasswordBtn.disabled = true;
      setMFAStatus('Saving local password...');
      try {
        await api('/auth/local-password/set', {
          method: 'POST',
          csrf: true,
          body: JSON.stringify({ password }),
        });
        el.mfaLocalPasswordInput.value = '';
        el.mfaLocalPasswordConfirmInput.value = '';
        el.mfaLocalPasswordConfirmInput.classList.add('hidden');
        await loadMFAStatus();
        setMFAStatus('Local password updated.');
      } catch (err) {
        setMFAStatus(err.message, true);
      } finally {
        updateLocalPasswordActions();
      }
    }

    async function disableLocalPasswordFromSecurity() {
      if (!state.user) return;
      const confirmed = window.confirm('Disable local password fallback?');
      if (!confirmed) return;
      el.disableLocalPasswordBtn.disabled = true;
      setMFAStatus('Disabling local password...');
      try {
        await api('/auth/local-password', { method: 'DELETE', csrf: true });
        el.mfaLocalPasswordInput.value = '';
        el.mfaLocalPasswordConfirmInput.value = '';
        el.mfaLocalPasswordConfirmInput.classList.add('hidden');
        await loadMFAStatus();
        setMFAStatus('Local password disabled.');
      } catch (err) {
        setMFAStatus(err.message, true);
      }
    }

    async function disableTotpFromSecurity() {
      if (!state.user) return;
      const confirmed = window.confirm('Disable TOTP for this account?');
      if (!confirmed) return;
      el.disableTotpBtn.disabled = true;
      setMFAStatus('Disabling TOTP...');
      try {
        await api('/auth/mfa/totp', { method: 'DELETE', csrf: true });
        await loadMFAStatus();
        setMFAStatus('TOTP disabled.');
      } catch (err) {
        setMFAStatus(err.message, true);
      }
    }

    async function renamePasskeyFromSecurity(credentialID, displayName) {
      if (!state.user) return;
      const cleanCredentialID = String(credentialID || '').trim();
      const cleanDisplayName = String(displayName || '').trim() || 'Passkey';
      if (!cleanCredentialID) {
        setMFAStatus(humanizeError('webauthn_credential_required'), true);
        return;
      }
      setMFAStatus('Saving passkey name...');
      try {
        await api(`/auth/mfa/webauthn/${encodeURIComponent(cleanCredentialID)}`, {
          method: 'PATCH',
          csrf: true,
          body: JSON.stringify({ display_name: cleanDisplayName }),
        });
        await loadMFAStatus();
        setMFAStatus('Passkey name updated.');
      } catch (err) {
        setMFAStatus(err.message, true);
      }
    }

    async function deletePasskeyFromSecurity(credentialID) {
      if (!state.user) return;
      const cleanCredentialID = String(credentialID || '').trim();
      if (!cleanCredentialID) {
        setMFAStatus(humanizeError('webauthn_credential_required'), true);
        return;
      }
      const confirmed = window.confirm('Delete this passkey?');
      if (!confirmed) return;
      setMFAStatus('Deleting passkey...');
      try {
        await api(`/auth/mfa/webauthn/${encodeURIComponent(cleanCredentialID)}`, {
          method: 'DELETE',
          csrf: true,
        });
        await loadMFAStatus();
        setMFAStatus('Passkey deleted.');
      } catch (err) {
        setMFAStatus(err.message, true);
      }
    }

    function handlePasskeyListClick(event) {
      const target = event.target;
      if (!(target instanceof HTMLElement)) return;
      const renameID = target.getAttribute('data-passkey-rename');
      if (renameID) {
        const nameInput = el.mfaPasskeysBody.querySelector(`input[data-passkey-name="${renameID}"]`);
        const value = nameInput instanceof HTMLInputElement ? nameInput.value : '';
        renamePasskeyFromSecurity(renameID, value);
        return;
      }
      const deleteID = target.getAttribute('data-passkey-delete');
      if (deleteID) {
        deletePasskeyFromSecurity(deleteID);
      }
    }

    function handlePasskeyListKeydown(event) {
      if (event.key !== 'Enter') return;
      const target = event.target;
      if (!(target instanceof HTMLInputElement)) return;
      const credentialID = target.getAttribute('data-passkey-name');
      if (!credentialID) return;
      renamePasskeyFromSecurity(credentialID, target.value);
    }

    async function sha256Hex(file, onProgress) {
      const chunkSize = 8 * 1024 * 1024;
      const sha = new SHA256();
      let offset = 0;
      while (offset < file.size) {
        const end = Math.min(offset + chunkSize, file.size);
        let chunkBuffer;
        try {
          chunkBuffer = await file.slice(offset, end).arrayBuffer();
        } catch (err) {
          debugUpload('upload:file-read-error', {
            file: file && file.name ? file.name : '',
            offset,
            end,
            name: err && err.name ? err.name : '',
            message: err && err.message ? err.message : '',
          });
          const wrapped = new Error('file_read_failed');
          wrapped.cause = err;
          throw wrapped;
        }
        const chunk = new Uint8Array(chunkBuffer);
        sha.update(chunk);
        offset = end;
        if (onProgress) onProgress({ loaded: offset, total: file.size });
        await yieldToBrowser();
      }
      return sha.hex();
    }

    function normalizeUploadError(err) {
      if (!err) return new Error('upload_failed');
      if (err.message === 'upload_canceled' || err.name === 'AbortError' || err.code === 'upload_part_canceled') {
        return new Error('upload_canceled');
      }
      if (err.message === 'network_upload_failed' || err.message === 'file_read_failed') {
        return err;
      }
      if (err instanceof TypeError) {
        return new Error('network_upload_failed');
      }
      if (typeof err.message === 'string' && err.message.trim()) {
        return err;
      }
      return new Error('upload_failed');
    }

    function buildUploadBody(file, start, end) {
      const blob = file.slice(start, end);
      return { body: blob, mode: 'blob' };
    }

    function browserTransportChunkSize() {
      return 24 * 1024 * 1024;
    }

    function uploadPartXHR(url, body, options = {}) {
      const csrf = options.csrf || '';
      const onProgress = typeof options.onProgress === 'function' ? options.onProgress : null;
      return new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open('POST', url, true);
        xhr.withCredentials = true;
        xhr.responseType = 'text';
        xhr.setRequestHeader('Content-Type', 'application/octet-stream');
        xhr.setRequestHeader('X-CSRF-Token', csrf);
        Object.entries(options.headers || {}).forEach(([name, value]) => {
          xhr.setRequestHeader(name, String(value));
        });
        if (options.abortHandle && typeof options.abortHandle === 'object') {
          options.abortHandle.abort = () => xhr.abort();
        }
        xhr.upload.onprogress = (event) => {
          if (!onProgress) return;
          if (event && event.lengthComputable) onProgress({ loaded: event.loaded, total: event.total });
        };
        xhr.onerror = () => reject(uploadXHRFailure('upload_part_network_failed', xhr, 'error'));
        xhr.onabort = () => reject(uploadXHRFailure('upload_part_canceled', xhr, 'abort'));
        xhr.ontimeout = () => reject(uploadXHRFailure('upload_part_timeout', xhr, 'timeout'));
        xhr.onload = () => {
          let parsed = null;
          const text = typeof xhr.responseText === 'string' ? xhr.responseText : '';
          if (text) {
            try {
              parsed = JSON.parse(text);
            } catch (_) {}
          }
          resolve({
            ok: xhr.status >= 200 && xhr.status < 300,
            status: xhr.status,
            statusText: xhr.statusText || '',
            body: parsed,
          });
        };
        xhr.send(body);
      });
    }

    async function uploadPartChunked(uploadID, partNumber, file, start, end, options = {}) {
      const total = end - start;
      const chunkSize = Math.max(1024 * 1024, Number(options.chunkSize) || browserTransportChunkSize());
      const chunkCount = Math.max(1, Math.ceil(total / chunkSize));
      let offset = 0;
      let response = null;
      while (offset < total) {
        const index = Math.floor(offset / chunkSize);
        const chunkStart = start + offset;
        const chunkEnd = Math.min(end, chunkStart + chunkSize);
        const final = chunkEnd >= end;
        debugUpload('part:chunk-send', {
          upload_id: uploadID,
          part: partNumber,
          chunk: index + 1,
          chunks: chunkCount,
          offset,
          size: chunkEnd - chunkStart,
          final,
        });
        response = await uploadPartXHR(`/uploads/${uploadID}/parts/${partNumber}/chunks`, file.slice(chunkStart, chunkEnd), {
          csrf: options.csrf,
          abortHandle: options.abortHandle,
          headers: {
            'X-TeleVault-Chunk-Offset': offset,
            'X-TeleVault-Chunk-Final': final ? 'true' : 'false',
          },
          onProgress: ({ loaded }) => {
            if (options.onProgress) {
              options.onProgress({
                loaded: Math.min(total, offset + loaded),
                total,
                chunkIndex: index + 1,
                chunkCount,
              });
            }
          },
        });
        if (!response.ok && response.status !== 202) {
          const body = response.body || {};
          if (response.status === 409 && body.error === 'upload_chunk_offset_mismatch') {
            const expectedOffset = Number(body.expected_offset);
            if (Number.isFinite(expectedOffset) && expectedOffset >= 0 && expectedOffset <= total && expectedOffset !== offset) {
              debugUpload('part:chunk-resume-offset', {
                upload_id: uploadID,
                part: partNumber,
                expected_offset: expectedOffset,
                previous_offset: offset,
              });
              offset = expectedOffset;
              continue;
            }
          }
          return response;
        }
        const body = response.body || {};
        const nextOffset = Number(body.next_offset);
        const fallbackOffset = chunkEnd - start;
        if (Number.isFinite(nextOffset) && nextOffset > offset && nextOffset <= total) {
          offset = nextOffset;
        } else {
          offset = fallbackOffset;
        }
      }
      return response || {
        ok: false,
        status: 0,
        statusText: '',
        body: { error: 'upload_part_http_failed' },
      };
    }

    function uploadXHRFailure(code, xhr, eventName) {
      const err = new Error(humanizeError(code));
      err.code = code;
      err.status = xhr && Number.isFinite(Number(xhr.status)) ? Number(xhr.status) : 0;
      err.statusText = xhr && xhr.statusText ? xhr.statusText : '';
      err.event = eventName;
      return err;
    }

    function yieldToBrowser() {
      return new Promise((resolve) => setTimeout(resolve, 0));
    }

    class SHA256 {
      constructor() {
        this.h = new Uint32Array([
          0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a,
          0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19,
        ]);
        this.buffer = new Uint8Array(64);
        this.bufferLength = 0;
        this.bytesHashed = 0;
        this.finished = false;
        this.k = SHA256.K;
      }

      update(data) {
        if (this.finished) throw new Error('sha256 already finalized');
        let position = 0;
        this.bytesHashed += data.length;
        while (position < data.length) {
          const take = Math.min(data.length - position, 64 - this.bufferLength);
          this.buffer.set(data.subarray(position, position + take), this.bufferLength);
          this.bufferLength += take;
          position += take;
          if (this.bufferLength === 64) {
            this.transform(this.buffer);
            this.bufferLength = 0;
          }
        }
        return this;
      }

      hex() {
        const digest = this.digest();
        return Array.from(digest).map((byte) => byte.toString(16).padStart(2, '0')).join('');
      }

      digest() {
        if (!this.finished) {
          const bytesHashed = this.bytesHashed;
          this.buffer[this.bufferLength++] = 0x80;
          if (this.bufferLength > 56) {
            this.buffer.fill(0, this.bufferLength, 64);
            this.transform(this.buffer);
            this.bufferLength = 0;
          }
          this.buffer.fill(0, this.bufferLength, 56);
          const bitsHashed = bytesHashed * 8;
          const high = Math.floor(bitsHashed / 0x100000000);
          const low = bitsHashed >>> 0;
          this.buffer[56] = high >>> 24;
          this.buffer[57] = high >>> 16;
          this.buffer[58] = high >>> 8;
          this.buffer[59] = high;
          this.buffer[60] = low >>> 24;
          this.buffer[61] = low >>> 16;
          this.buffer[62] = low >>> 8;
          this.buffer[63] = low;
          this.transform(this.buffer);
          this.finished = true;
        }
        const out = new Uint8Array(32);
        for (let i = 0; i < 8; i++) {
          out[i * 4] = this.h[i] >>> 24;
          out[i * 4 + 1] = this.h[i] >>> 16;
          out[i * 4 + 2] = this.h[i] >>> 8;
          out[i * 4 + 3] = this.h[i];
        }
        return out;
      }

      transform(chunk) {
        const w = new Uint32Array(64);
        for (let i = 0; i < 16; i++) {
          const j = i * 4;
          w[i] = ((chunk[j] << 24) | (chunk[j + 1] << 16) | (chunk[j + 2] << 8) | chunk[j + 3]) >>> 0;
        }
        for (let i = 16; i < 64; i++) {
          const s0 = rotr(w[i - 15], 7) ^ rotr(w[i - 15], 18) ^ (w[i - 15] >>> 3);
          const s1 = rotr(w[i - 2], 17) ^ rotr(w[i - 2], 19) ^ (w[i - 2] >>> 10);
          w[i] = (w[i - 16] + s0 + w[i - 7] + s1) >>> 0;
        }

        let a = this.h[0], b = this.h[1], c = this.h[2], d = this.h[3];
        let e = this.h[4], f = this.h[5], g = this.h[6], h = this.h[7];
        for (let i = 0; i < 64; i++) {
          const s1 = rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25);
          const ch = (e & f) ^ (~e & g);
          const temp1 = (h + s1 + ch + this.k[i] + w[i]) >>> 0;
          const s0 = rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22);
          const maj = (a & b) ^ (a & c) ^ (b & c);
          const temp2 = (s0 + maj) >>> 0;
          h = g;
          g = f;
          f = e;
          e = (d + temp1) >>> 0;
          d = c;
          c = b;
          b = a;
          a = (temp1 + temp2) >>> 0;
        }

        this.h[0] = (this.h[0] + a) >>> 0;
        this.h[1] = (this.h[1] + b) >>> 0;
        this.h[2] = (this.h[2] + c) >>> 0;
        this.h[3] = (this.h[3] + d) >>> 0;
        this.h[4] = (this.h[4] + e) >>> 0;
        this.h[5] = (this.h[5] + f) >>> 0;
        this.h[6] = (this.h[6] + g) >>> 0;
        this.h[7] = (this.h[7] + h) >>> 0;
      }
    }

    SHA256.K = new Uint32Array([
      0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
      0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
      0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
      0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
      0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
      0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
      0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
      0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
    ]);

    function rotr(value, bits) {
      return (value >>> bits) | (value << (32 - bits));
    }

    function formatBytes(bytes) {
      if (!bytes) return '0 B';
      const units = ['B', 'KB', 'MB', 'GB', 'TB'];
      const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
      return `${(bytes / Math.pow(1024, index)).toFixed(index ? 1 : 0)} ${units[index]}`;
    }

    function formatPartCount(count) {
      if (!Number.isInteger(count) || count < 0) return '-';
      return `${count} part${count === 1 ? '' : 's'}`;
    }

    function formatETA(seconds) {
      if (!Number.isFinite(Number(seconds)) || Number(seconds) <= 0) return 'unknown';
      seconds = Math.ceil(Number(seconds));
      const hours = Math.floor(seconds / 3600);
      const minutes = Math.floor((seconds % 3600) / 60);
      const rest = seconds % 60;
      if (hours > 0) return `${hours}h ${minutes}m`;
      if (minutes > 0) return `${minutes}m ${rest}s`;
      return `${rest}s`;
    }

    function formatSpeed(bytesPerSecond) {
      const value = Number(bytesPerSecond);
      if (!Number.isFinite(value) || value <= 0) return 'speed unknown';
      return `${formatBytes(value)}/s`;
    }

    function formatRetryAt(value) {
      if (!value) return '';
      const retryAt = new Date(value).getTime();
      if (!Number.isFinite(retryAt)) return '';
      const seconds = Math.max(0, Math.ceil((retryAt - Date.now()) / 1000));
      return seconds > 0 ? `in ${formatETA(seconds)}` : 'now';
    }

    function escapeHTML(value) {
      return String(value).replace(/[&<>"']/g, (char) => ({
        '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
      }[char]));
    }

    function delay(ms) {
      return new Promise((resolve) => setTimeout(resolve, ms));
    }

    function hasExternalFiles(dataTransfer) {
      return dataTransfer && Array.from(dataTransfer.types || []).includes('Files');
    }

    function getDataTransferEntries(dataTransfer) {
      if (!dataTransfer || !dataTransfer.items || !dataTransfer.items.length) return [];
      const entries = [];
      for (const item of Array.from(dataTransfer.items)) {
        if (!item || item.kind !== 'file' || typeof item.webkitGetAsEntry !== 'function') continue;
        const entry = item.webkitGetAsEntry();
        if (entry) entries.push(entry);
      }
      return entries;
    }

    function readAllDirectoryEntries(dirEntry) {
      return new Promise((resolve, reject) => {
        const reader = dirEntry.createReader();
        const all = [];
        const readBatch = () => {
          reader.readEntries((batch) => {
            if (!batch.length) {
              resolve(all);
              return;
            }
            all.push(...batch);
            readBatch();
          }, reject);
        };
        readBatch();
      });
    }

    function readFileEntry(fileEntry) {
      return new Promise((resolve, reject) => {
        fileEntry.file(resolve, reject);
      });
    }

    async function collectEntryUploads(entry, relPath = '') {
      if (!entry) return [];
      if (entry.isFile) {
        const file = await readFileEntry(entry);
        if (!file || file.size < 0) return [];
        return [{
          file,
          pathParts: relPath ? relPath.split('/').filter(Boolean) : [],
        }];
      }
      if (!entry.isDirectory) return [];
      const children = await readAllDirectoryEntries(entry);
      let uploads = [];
      for (const child of children) {
        const childRel = relPath ? `${relPath}/${child.name}` : child.name;
        uploads = uploads.concat(await collectEntryUploads(child, childRel));
      }
      return uploads;
    }

    async function collectDroppedUploads(dataTransfer) {
      const entries = getDataTransferEntries(dataTransfer);
      if (entries.length) {
        let uploads = [];
        for (const entry of entries) {
          uploads = uploads.concat(await collectEntryUploads(entry, entry.name));
        }
        if (uploads.length) return uploads;
      }
      return Array.from((dataTransfer && dataTransfer.files) || [])
        .filter((file) => file && file.size >= 0)
        .map((file) => ({ file, pathParts: [] }));
    }

    async function ensureFolderByPath(parentID, pathParts, cache) {
      let currentParent = parentID || '';
      const cleanParts = Array.isArray(pathParts) ? pathParts.filter(Boolean) : [];
      for (const part of cleanParts) {
        const key = `${currentParent}|${part}`;
        if (cache.has(key)) {
          currentParent = cache.get(key);
          continue;
        }
        const created = await api('/folders', {
          method: 'POST',
          csrf: true,
          body: JSON.stringify({
            name: part,
            parent_id: currentParent || undefined,
          }),
        });
        const folderID = created && created.file && created.file.id ? created.file.id : '';
        if (!folderID) throw new Error('folder_create_failed');
        cache.set(key, folderID);
        currentParent = folderID;
      }
      return currentParent;
    }

    async function enqueueDroppedStructure(dataTransfer, parentID) {
      if (!requireWritableAction()) return { queued: 0, resumed: 0 };
      const uploads = await collectDroppedUploads(dataTransfer);
      if (!uploads.length) return { queued: 0, resumed: 0 };
      const folderCache = new Map();
      const queueEntries = [];
      for (const upload of uploads) {
        const folderParts = upload.pathParts.slice(0, -1);
        const resolvedParentID = await ensureFolderByPath(parentID || '', folderParts, folderCache);
        queueEntries.push({
          file: upload.file,
          parentID: resolvedParentID,
          displayPath: upload.pathParts.join('/'),
        });
      }
      const attached = attachFilesToPendingResumes(queueEntries);
      if (attached.unmatched.length) enqueueFiles(attached.unmatched, parentID || '');
      return { queued: attached.unmatched.length, resumed: attached.resumed };
    }

    function setAdminStatus(message, error = false) {
      el.adminStatus.textContent = message;
      el.adminStatus.classList.toggle('error', error);
    }

    async function openAdminPanel() {
      if (!state.user || state.user.role !== 'admin') return;
      el.adminModal.classList.remove('hidden');
      setAdminStatus('Loading...');
      try {
        const data = await api('/admin/settings');
        fillAdminUploadSettings(data.upload_settings || {});
        fillAdminLicense(data.instance_id || '', data.license || {});
        setAdminStatus('Loaded.');
      } catch (err) {
        setAdminStatus(err.message, true);
      }
    }

    function closeAdminPanel() {
      el.adminModal.classList.add('hidden');
    }

    function setAdminLicenseStatus(message, error = false) {
      el.adminLicenseStatus.textContent = message;
      el.adminLicenseStatus.classList.toggle('error', error);
    }

    function fillAdminUploadSettings(settings) {
      el.adminPartSizeInput.value = settings.upload_part_size_bytes || '';
      el.adminDocumentLimitInput.value = settings.telegram_document_limit_bytes || '';
      el.adminSafetyMarginInput.value = settings.upload_safety_margin_bytes || 0;
      el.adminParallelInput.value = settings.max_parallel_uploads || 1;
      el.adminRateInput.value = settings.target_upload_bytes_per_second || 0;
      el.adminCooldownInput.value = settings.cooldown_between_parts_ms || 0;
      el.adminPublicPasswordMinInput.value = settings.public_link_password_min_length || 8;
      if (!state.appInfo) state.appInfo = {};
      state.appInfo.public_link_password_min_length = Number(el.adminPublicPasswordMinInput.value) || 8;
      applyPublicLinkPasswordPolicy();
    }

    function fillAdminLicense(instanceID, license) {
      state.adminInstanceID = instanceID || '';
      state.adminLicense = license || null;
      el.adminInstanceIDInput.value = instanceID || '';
      const status = license && license.status ? String(license.status) : 'missing';
      const tier = license && license.tier ? String(license.tier) : 'community';
      const expires = license && license.expires_at ? formatDate(license.expires_at) : 'n/a';
      const maxAccounts = Number(license && license.max_connected_telegram_accounts || 1);
      const maxWorkspaces = Number(license && license.max_workspaces || 1);
      const instanceMatch = license && Object.prototype.hasOwnProperty.call(license, 'instance_match')
        ? (license.instance_match ? 'match' : 'mismatch')
        : 'n/a';
      const validationError = license && license.validation_error ? ` (${license.validation_error})` : '';

      el.adminLicenseStatusInput.value = `${status}${validationError}`;
      el.adminLicenseTierInput.value = `${tier} (effective: ${license && license.effective_edition ? license.effective_edition : 'community'})`;
      el.adminLicenseLimitsInput.value = `${maxWorkspaces} workspaces, ${maxAccounts} accounts`;
      el.adminLicenseExpiresInput.value = expires;
      el.adminLicenseInstanceMatchInput.value = instanceMatch;
      el.adminLicenseSummaryBadge.textContent = status;
      el.adminLicenseSummaryBadge.classList.remove('ok', 'warn', 'error');
      if (status === 'valid') {
        el.adminLicenseSummaryBadge.classList.add('ok');
      } else if (status === 'invalid' || status === 'expired' || status === 'instance_mismatch') {
        el.adminLicenseSummaryBadge.classList.add('error');
      } else {
        el.adminLicenseSummaryBadge.classList.add('warn');
      }
    }

    function adminNumber(input, name) {
      const value = Number(input.value);
      if (!Number.isFinite(value) || value < 0) throw new Error(`${name} is invalid`);
      return Math.floor(value);
    }

    async function saveAdminUploadSettings() {
      if (!state.user || state.user.role !== 'admin') return;
      setAdminStatus('Saving...');
      el.saveAdminUploadSettingsBtn.disabled = true;
      try {
        const payload = {
          upload_part_size_bytes: adminNumber(el.adminPartSizeInput, 'Part size'),
          telegram_document_limit_bytes: adminNumber(el.adminDocumentLimitInput, 'Telegram document limit'),
          upload_safety_margin_bytes: adminNumber(el.adminSafetyMarginInput, 'Safety margin'),
          max_parallel_uploads: adminNumber(el.adminParallelInput, 'Parallel uploads'),
          target_upload_bytes_per_second: adminNumber(el.adminRateInput, 'Target rate'),
          cooldown_between_parts_ms: adminNumber(el.adminCooldownInput, 'Cooldown'),
          public_link_password_min_length: adminNumber(el.adminPublicPasswordMinInput, 'Public password min length'),
        };
        const data = await api('/admin/settings/upload', {
          method: 'PATCH',
          csrf: true,
          body: JSON.stringify(payload),
        });
        const saved = data.upload_settings || payload;
        fillAdminUploadSettings(saved);
        if (!state.appInfo) state.appInfo = {};
        state.appInfo.public_link_password_min_length = saved.public_link_password_min_length;
        applyPublicLinkPasswordPolicy();
        setAdminStatus('Upload policy saved.');
      } catch (err) {
        setAdminStatus(err.message, true);
      } finally {
        el.saveAdminUploadSettingsBtn.disabled = false;
      }
    }

    async function saveAdminLicense() {
      if (!state.user || state.user.role !== 'admin') return;
      const raw = el.adminLicenseRawInput.value.trim();
      if (!raw) {
        setAdminLicenseStatus('License JSON is required.', true);
        return;
      }
      setAdminLicenseStatus('Installing...');
      el.saveAdminLicenseBtn.disabled = true;
      try {
        const data = await api('/admin/settings/license', {
          method: 'PATCH',
          csrf: true,
          body: JSON.stringify({ raw_license_json: raw }),
        });
        fillAdminLicense(data.instance_id || state.adminInstanceID, data.license || {});
        setAdminLicenseStatus('License installed.');
      } catch (err) {
        setAdminLicenseStatus(err.message, true);
      } finally {
        el.saveAdminLicenseBtn.disabled = false;
      }
    }

    async function removeAdminLicense() {
      if (!state.user || state.user.role !== 'admin') return;
      if (!window.confirm('Remove installed license and fallback to Community?')) return;
      setAdminLicenseStatus('Removing...');
      el.removeAdminLicenseBtn.disabled = true;
      try {
        const data = await api('/admin/settings/license', {
          method: 'DELETE',
          csrf: true,
        });
        fillAdminLicense(data.instance_id || state.adminInstanceID, data.license || {});
        el.adminLicenseRawInput.value = '';
        setAdminLicenseStatus('License removed.');
      } catch (err) {
        setAdminLicenseStatus(err.message, true);
      } finally {
        el.removeAdminLicenseBtn.disabled = false;
      }
    }

    function setDetailsStatus(message, error = false) {
      el.detailsStatus.textContent = message;
      el.detailsStatus.classList.toggle('error', error);
    }

    function visibleFileIDs() {
      return Array.from(el.filesBody.querySelectorAll('[data-select-file]'))
        .map((input) => input.dataset.selectFile)
        .filter(Boolean);
    }

    function clearSelection() {
      state.selectedFileIds.clear();
      state.selectionAnchorID = '';
      renderSelectionBar(visibleFileIDs().map((id) => ({ id })));
      syncSelectionCheckboxes();
    }

    function toggleVisibleSelection() {
      const visibleIDs = visibleFileIDs();
      if (!visibleIDs.length) return;
      const visibleSelected = visibleIDs.every((id) => state.selectedFileIds.has(id));
      if (visibleSelected) {
        visibleIDs.forEach((id) => state.selectedFileIds.delete(id));
        state.selectionAnchorID = '';
      } else {
        visibleIDs.forEach((id) => state.selectedFileIds.add(id));
        state.selectionAnchorID = visibleIDs[visibleIDs.length - 1] || '';
      }
      syncSelectionCheckboxes();
      renderSelectionBar(visibleIDs.map((id) => ({ id })));
    }

    function syncSelectionCheckboxes() {
      el.filesBody.querySelectorAll('[data-select-file]').forEach((input) => {
        input.checked = state.selectedFileIds.has(input.dataset.selectFile);
      });
    }

    el.startQrBtn.addEventListener('click', startQR);
    el.continueRememberedBtn.addEventListener('click', continueRememberedLogin);
    el.forgetRememberedBtn.addEventListener('click', forgetRememberedDevice);
    el.useAnotherAccountBtn.addEventListener('click', useAnotherAccount);
    el.sendCodeBtn.addEventListener('click', sendTelegramCode);
    el.loginWithCodeBtn.addEventListener('click', loginWithCode);
    el.loginUseWebauthnBtn.addEventListener('click', async () => {
      try {
        const verified = await tryWebAuthnLocalMFAVerify();
        if (!verified) {
          if (hasMethod('totp') || hasMethod('recovery')) {
            showLocalCodeVerification(true);
          } else if (hasMethod('password')) {
            showLocalPasswordVerification();
          } else {
            el.loginStatus.classList.add('error');
            el.loginStatus.textContent = 'Passkey is required for this account.';
          }
        }
      } catch (err) {
        el.loginStatus.classList.add('error');
        if (isWebAuthnUserAgentDenied(err)) {
          el.loginStatus.textContent = 'Passkey request was denied or cancelled. Use 2FA code, recovery key, or local password.';
        } else {
          el.loginStatus.textContent = err.message;
        }
      }
    });
    el.loginUsePasswordBtn.addEventListener('click', () => showLocalPasswordVerification());
    el.localPasswordToggleBtn.addEventListener('click', toggleLocalPasswordVisibility);
    el.loginCountrySelect.addEventListener('change', () => {
      clearTelegramCodeRetryState();
      state.phoneCountry = String(el.loginCountrySelect.value || '').trim() || state.phoneCountry;
      updatePhoneCountryHint();
      refreshPhonePreview();
    });
    el.loginPhoneInput.addEventListener('input', () => {
      clearTelegramCodeRetryState();
      refreshPhonePreview();
    });
    el.loginPhoneInput.addEventListener('paste', () => {
      clearTelegramCodeRetryState();
      setTimeout(refreshPhonePreview, 0);
    });
    el.loginPhoneInput.addEventListener('keydown', (event) => {
      if (event.key === 'Enter') sendTelegramCode();
    });
    el.loginCodeInput.addEventListener('keydown', (event) => {
      if (event.key === 'Enter') loginWithCode();
    });
    el.loginPasswordInput.addEventListener('keydown', (event) => {
      if (event.key === 'Enter') loginWithCode();
    });
    el.localPasswordInput.addEventListener('keydown', (event) => {
      if (event.key === 'Enter') loginWithCode();
    });
    el.logoutBtn.addEventListener('click', () => logout(false));
    el.logoutForgetBtn.addEventListener('click', () => logout(true));
    el.reconnectTelegramBtn.addEventListener('click', startReconnectTelegram);
    el.securityBtn.addEventListener('click', openSecurityDialog);
    el.adminBtn.addEventListener('click', openAdminPanel);
    el.refreshBtn.addEventListener('click', refreshFiles);
    el.ownFilesBtn.addEventListener('click', () => setView('own'));
    el.sharedFilesBtn.addEventListener('click', () => setView('shared'));
    el.upBtn.addEventListener('click', goUp);
    el.upBtn.addEventListener('dragover', (event) => {
      if (!state.draggingItems.length || state.view !== 'own' || !state.currentFolderId) return;
      event.preventDefault();
      event.dataTransfer.dropEffect = 'move';
      el.upBtn.classList.add('drag-over');
    });
    el.upBtn.addEventListener('dragleave', () => el.upBtn.classList.remove('drag-over'));
    el.upBtn.addEventListener('drop', async (event) => {
      if (!state.draggingItems.length || state.view !== 'own' || !state.currentFolderId) return;
      event.preventDefault();
      el.upBtn.classList.remove('drag-over');
      const parent = state.folderStack[state.folderStack.length - 2];
      await moveFiles(state.draggingItems, parent ? parent.id : '');
    });
    el.createFolderBtn.addEventListener('click', createFolder);
    el.folderNameInput.addEventListener('keydown', (event) => {
      if (event.key === 'Enter') createFolder();
    });
    el.selectAllVisibleBtn.addEventListener('click', toggleVisibleSelection);
    el.moveSelectedBtn.addEventListener('click', openMoveDialog);
    el.deleteSelectedBtn.addEventListener('click', () => deleteFiles(Array.from(state.selectedFileIds)));
    el.clearSelectionBtn.addEventListener('click', clearSelection);
    el.dropZone.addEventListener('click', () => {
      if (!requireWritableAction()) return;
      el.fileInput.click();
    });
    el.dropZone.addEventListener('keydown', (event) => {
      if (event.key === 'Enter' || event.key === ' ') {
        if (!requireWritableAction()) return;
        el.fileInput.click();
      }
    });
    el.fileInput.addEventListener('change', () => {
      state.droppedFiles = [];
      updateDropZone();
    });
    el.dropZone.addEventListener('dragover', (event) => {
      event.preventDefault();
      el.dropZone.classList.add('dragging');
    });
    el.dropZone.addEventListener('dragleave', () => {
      el.dropZone.classList.remove('dragging');
    });
    el.dropZone.addEventListener('drop', async (event) => {
      event.preventDefault();
      el.dropZone.classList.remove('dragging');
      try {
        const result = await enqueueDroppedStructure(event.dataTransfer, state.currentFolderId || '');
        if (!result.queued && !result.resumed) {
          acceptDroppedFiles(event.dataTransfer.files);
          return;
        }
        state.droppedFiles = [];
        el.fileInput.value = '';
        updateDropZone();
        if (result.queued && result.resumed) {
          setUploadStatus(`Queued ${result.queued} file${result.queued === 1 ? '' : 's'}, resumed ${result.resumed}.`);
        } else if (result.queued) {
          setUploadStatus(`Queued ${result.queued} file${result.queued === 1 ? '' : 's'}.`);
        } else {
          setUploadStatus(`Resumed ${result.resumed} file${result.resumed === 1 ? '' : 's'}.`);
        }
        runUploadQueue();
      } catch (err) {
        setUploadStatus(normalizeUploadError(err).message, true);
      }
    });
    el.filePanel.addEventListener('dragover', (event) => {
      if (state.draggingItems.length || state.view !== 'own') return;
      if (!hasExternalFiles(event.dataTransfer)) return;
      event.preventDefault();
      el.filePanel.classList.add('dragging');
    });
    el.filePanel.addEventListener('dragleave', (event) => {
      if (!el.filePanel.contains(event.relatedTarget)) el.filePanel.classList.remove('dragging');
    });
    el.filePanel.addEventListener('drop', async (event) => {
      if (state.draggingItems.length || state.view !== 'own') return;
      if (!hasExternalFiles(event.dataTransfer)) return;
      event.preventDefault();
      el.filePanel.classList.remove('dragging');
      try {
        const result = await enqueueDroppedStructure(event.dataTransfer, state.currentFolderId || '');
        if (!result.queued && !result.resumed) return;
        if (result.queued && result.resumed) {
          setUploadStatus(`Queued ${result.queued} file${result.queued === 1 ? '' : 's'} for this folder, resumed ${result.resumed}.`);
        } else if (result.queued) {
          setUploadStatus(`Queued ${result.queued} file${result.queued === 1 ? '' : 's'} for this folder.`);
        } else {
          setUploadStatus(`Resumed ${result.resumed} file${result.resumed === 1 ? '' : 's'} for this folder.`);
        }
        runUploadQueue();
      } catch (err) {
        setUploadStatus(normalizeUploadError(err).message, true);
      }
    });
    function acceptDroppedFiles(fileList) {
      state.droppedFiles = Array.from(fileList || []).filter((file) => file && file.size >= 0);
      el.fileInput.value = '';
      updateDropZone();
    }
    el.uploadBtn.addEventListener('click', uploadSelected);
    el.clearCompletedQueueBtn.addEventListener('click', clearCompletedQueueItems);
    el.uploadDebugToggle.addEventListener('change', () => setUploadDebugEnabled(el.uploadDebugToggle.checked));
    el.exportRecoveryBtn.addEventListener('click', exportRecovery);
    el.importRecoveryBtn.addEventListener('click', startRecoveryImport);
    el.recoveryFileInput.addEventListener('change', importRecoveryFile);
    el.closeDetailsBtn.addEventListener('click', closeDetailsDialog);
    el.closeSecurityBtn.addEventListener('click', closeSecurityDialog);
    el.startTotpEnrollBtn.addEventListener('click', startTotpEnrollmentFromSecurity);
    el.confirmTotpEnrollBtn.addEventListener('click', confirmTotpEnrollmentFromSecurity);
    el.registerWebauthnBtn.addEventListener('click', registerWebauthnFromSecurity);
    el.mfaPasskeysBody.addEventListener('click', handlePasskeyListClick);
    el.mfaPasskeysBody.addEventListener('keydown', handlePasskeyListKeydown);
    el.disableTotpBtn.addEventListener('click', disableTotpFromSecurity);
    el.regenerateRecoveryBtn.addEventListener('click', regenerateRecoveryCodes);
    el.setLocalPasswordBtn.addEventListener('click', setLocalPasswordFromSecurity);
    el.disableLocalPasswordBtn.addEventListener('click', disableLocalPasswordFromSecurity);
    el.mfaLocalPasswordInput.addEventListener('input', updateLocalPasswordActions);
    el.mfaLocalPasswordConfirmInput.addEventListener('input', updateLocalPasswordActions);
    el.mfaTotpCodeInput.addEventListener('keydown', (event) => {
      if (event.key === 'Enter') confirmTotpEnrollmentFromSecurity();
    });
    el.mfaPasskeyNameInput.addEventListener('keydown', (event) => {
      if (event.key === 'Enter') registerWebauthnFromSecurity();
    });
    el.mfaLocalPasswordInput.addEventListener('keydown', (event) => {
      if (event.key === 'Enter') setLocalPasswordFromSecurity();
    });
    el.mfaLocalPasswordConfirmInput.addEventListener('keydown', (event) => {
      if (event.key === 'Enter' && !el.setLocalPasswordBtn.disabled) setLocalPasswordFromSecurity();
    });
    el.saveDetailsFileNameBtn.addEventListener('click', () => {
      if (!state.detailsFile) return;
      renameFile(state.detailsFile.id, el.detailsFileNameInput.value);
    });
    el.copyDetailsFileIDBtn.addEventListener('click', async () => {
      if (!el.detailsFileID.value) return;
      try {
        await navigator.clipboard.writeText(el.detailsFileID.value);
        setDetailsStatus('File ID copied.');
      } catch (_) {
        el.detailsFileID.focus();
        el.detailsFileID.select();
        setDetailsStatus('Select the file ID and copy it.');
      }
    });
    el.closeShareBtn.addEventListener('click', closeShareDialog);
    el.closeMoveBtn.addEventListener('click', closeMoveDialog);
    el.confirmMoveBtn.addEventListener('click', confirmMoveSelected);
    el.shareInternalTabBtn.addEventListener('click', () => {
      showShareTab('internal');
      focusShareTabInput('internal');
    });
    el.sharePublicTabBtn.addEventListener('click', () => {
      showShareTab('public');
      focusShareTabInput('public');
    });
    el.shareManualToggleBtn.addEventListener('click', () => {
      toggleShareManualInput();
      if (el.shareManualField.classList.contains('hidden')) {
        el.shareRecipientSelect.focus();
      } else {
        el.shareTelegramInput.focus();
      }
    });
    el.createShareBtn.addEventListener('click', createShare);
    el.createPublicLinkBtn.addEventListener('click', createPublicLink);
    el.copyPublicLinkBtn.addEventListener('click', copyPublicLink);
    el.closeAdminBtn.addEventListener('click', closeAdminPanel);
    el.saveAdminUploadSettingsBtn.addEventListener('click', saveAdminUploadSettings);
    el.saveAdminLicenseBtn.addEventListener('click', saveAdminLicense);
    el.removeAdminLicenseBtn.addEventListener('click', removeAdminLicense);
    el.pickAdminLicenseFileBtn.addEventListener('click', () => el.adminLicenseFileInput.click());
    el.adminLicenseFileInput.addEventListener('change', async () => {
      const file = el.adminLicenseFileInput.files && el.adminLicenseFileInput.files[0];
      if (!file) return;
      try {
        const text = await file.text();
        el.adminLicenseRawInput.value = text.trim();
        setAdminLicenseStatus(`Loaded ${file.name}.`);
      } catch (_) {
        setAdminLicenseStatus('Failed to read license file.', true);
      } finally {
        el.adminLicenseFileInput.value = '';
      }
    });
    el.copyAdminInstanceIDBtn.addEventListener('click', async () => {
      const value = el.adminInstanceIDInput.value.trim();
      if (!value) return;
      try {
        await navigator.clipboard.writeText(value);
        setAdminLicenseStatus('Instance ID copied.');
      } catch (_) {
        el.adminInstanceIDInput.focus();
        el.adminInstanceIDInput.select();
        setAdminLicenseStatus('Select instance ID and copy manually.', true);
      }
    });
    el.adminModal.addEventListener('click', (event) => {
      if (event.target === el.adminModal) closeAdminPanel();
    });
    el.securityModal.addEventListener('click', (event) => {
      if (event.target === el.securityModal) closeSecurityDialog();
    });
    el.shareModal.addEventListener('click', (event) => {
      if (event.target === el.shareModal) closeShareDialog();
    });
    el.moveModal.addEventListener('click', (event) => {
      if (event.target === el.moveModal) closeMoveDialog();
    });
    el.detailsModal.addEventListener('click', (event) => {
      if (event.target === el.detailsModal) closeDetailsDialog();
    });
    el.detailsFileNameInput.addEventListener('keydown', (event) => {
      if (event.key === 'Enter') {
        event.preventDefault();
        if (state.detailsFile) renameFile(state.detailsFile.id, el.detailsFileNameInput.value);
      }
    });
    document.addEventListener('keydown', (event) => {
      if (event.key !== 'Escape') return;
      if (!el.detailsModal.classList.contains('hidden')) {
        closeDetailsDialog();
        return;
      }
      if (!el.securityModal.classList.contains('hidden')) {
        closeSecurityDialog();
        return;
      }
      if (!el.adminModal.classList.contains('hidden')) {
        closeAdminPanel();
        return;
      }
      if (!el.shareModal.classList.contains('hidden')) {
        closeShareDialog();
        return;
      }
      if (!el.moveModal.classList.contains('hidden')) {
        closeMoveDialog();
        return;
      }
      if (hasActiveSelection()) {
        clearSelection();
      }
    });
    window.addEventListener('hashchange', applyRoute);
    setPhoneCountryOptions();
    setUploadDebugEnabled(state.uploadDebugEnabled);
    loadMe();
