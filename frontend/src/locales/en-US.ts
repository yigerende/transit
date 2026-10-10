export default {
  brand: {
    name: '监控探测',
    logoAlt: '监控探测 logo'
  },
  nav: {
    features: 'Features',
    integrations: 'Integrations',
    documentation: 'Documentation',
    pricing: 'Pricing',
    signIn: 'Sign In',
    getStarted: 'Get Started'
  },
  hero: {
    badge: 'Introducing 监控探测 2.0',
    title: 'The Ultimate',
    highlight: 'API Gateway',
    subtitle: 'Unify your NewAPI instances, manage keys with ease, and route traffic intelligently. Built for the modern AI infrastructure.',
    startBtn: 'Start Building Now',
    docBtn: 'View Documentation'
  },
  features: {
    title: 'Designed for Scale and Speed',
    subtitle: 'Everything you need to manage massive API traffic across distributed networks, packed into one beautiful interface.',
    items: {
      sync: {
        title: 'Multi-Instance Sync',
        desc: 'Seamlessly synchronize across multiple NewAPI instances with zero downtime and automatic conflict resolution.'
      },
      fallback: {
        title: 'Smart Fallback',
        desc: 'Intelligent routing and automatic fallback ensures your API requests never fail even if a provider goes down.'
      },
      observe: {
        title: 'Global Observability',
        desc: 'Monitor all your API keys, quotas, and latency metrics in real-time across the globe.'
      },
      selfhost: {
        title: 'Self-Hosted Ready',
        desc: 'Deploy anywhere. Full support for Docker, Kubernetes, and bare-metal VPS installations.'
      }
    }
  },
  cta: {
    title: 'Ready to take control?',
    subtitle: 'Join thousands of developers using 监控探测 to power their API infrastructure. Get started for free today.',
    deployBtn: 'Deploy Now',
    salesBtn: 'Contact Sales'
  },
  footer: {
    rights: '监控探测 Operations. All rights reserved.'
  },
  auth: {
    backToHome: 'Back to Home',
    login: {
      title: 'Welcome Back',
      subtitle: 'Enter your email and password to sign in',
      email: 'Email',
      emailPlaceholder: "name{'@'}example.com",
      password: 'Password',
      passwordPlaceholder: 'Enter your password',
      submit: 'Sign In',
      submitting: 'Signing in...',
      success: 'Signed in successfully. Opening the admin console...',
      errors: {
        login: 'Unable to sign in. Check your email and password, then try again.'
      },
      noAccount: 'Don\'t have an account?',
      registerLink: 'Register now'
    },
    register: {
      title: 'Create an Account',
      subtitle: 'Enter your details to register for 监控探测',
      email: 'Email',
      emailPlaceholder: "name{'@'}example.com",
      password: 'Password',
      passwordPlaceholder: 'Set a password',
      code: 'Verification Code',
      codePlaceholder: 'Enter 6-digit code',
      sendCode: 'Send Code',
      sendingCode: 'Sending...',
      codeSent: 'Sent',
      codeSentSuccess: 'Verification code sent. Use {code} to finish registration.',
      submit: 'Register',
      submitting: 'Registering...',
      success: 'Registration complete. Opening the admin console...',
      errors: {
        codeRequest: 'Unable to send the verification code. Check your email and try again.',
        register: 'Unable to register. Check the verification code and try again.'
      },
      hasAccount: 'Already have an account?',
      loginLink: 'Sign In'
    },
    errors: {
      emailRequired: 'Enter an email address first.',
      invalidRegister: 'Enter your email, password, and verification code.',
      invalidLogin: 'Enter your email and password.',
      invalidCode: 'The verification code is incorrect or expired.',
      emailExists: 'This email is already registered.',
      invalidCredentials: 'Email or password is incorrect.',
      unauthorized: 'Your session has expired. Please sign in again to continue.',
      registrationDisabled: 'Public registration is disabled for this deployment. Sign in with the admin account.',
      network: 'Network error. Check your connection and try again.',
      unknown: 'Something went wrong. Please try again.'
    }
  },
  admin: {
    layout: {
      toggleLanguage: 'Toggle language',
      toggleTheme: 'Toggle theme',
      userProfile: 'User profile',
      switchWorkspace: 'Switch Workspace',
      skipToContent: 'Skip to main content',
      openNavigation: 'Open navigation',
      closeNavigation: 'Close navigation'
    },
    menu: {
      dashboard: 'Dashboard',
      leaderboard: 'Leaderboard',
      lottery: 'Lottery',
      upstream: 'Upstream',
      groupManagement: 'Group Management',
      groupRates: 'Group Rates',
      groupAssociations: 'Pricing Mappings',
      connectionHealth: 'Group Health',
      groupRateCampaigns: 'Rate Campaigns',
      sub2apiFeatures: 'Embedded Features',
      settings: 'Settings',
      tickets: 'Tickets',
      massEmail: 'Mass Email',
      signOut: 'Sign Out'
    },
    leaderboard: {
      eyebrow: 'Usage ranking',
      title: 'Token Leaderboard',
      subtitle: 'Rank active workspace users by their actual token usage.',
      refresh: 'Refresh leaderboard',
      errorTitle: 'Unable to load leaderboard',
      emptyTitle: 'No usage in this period',
      emptyDescription: 'Choose another period or refresh again later.',
      anonymous: 'User {id}',
      podiumLabel: 'Top three users',
      updatedAt: 'Updated at {time}',
      period: { label: 'Reporting period', today: 'Today', '7d': '7 days', '30d': '30 days' },
      metrics: { tokens: 'Tokens', requests: 'Requests', cost: 'Actual spend' },
      table: { title: 'Full ranking', caption: '{count} users', rank: 'Rank', user: 'User' },
      embed: {
        action: 'Embed settings',
        title: 'Leaderboard embed settings',
        subtitle: 'The current workspace is bound automatically to create a secure Sub2API custom-menu URL.',
        sourceOrigin: 'Bound site',
        sourceOriginHint: 'This address comes from the current workspace Sub2API admin session and requires no manual input.',
        url: 'Iframe URL',
        urlHint: 'Use this URL in the Sub2API custom menu URL field.',
        copy: 'Copy URL',
        copyFailed: 'Copy failed. Select the URL manually.',
        rotate: 'Regenerate token',
        confirmRotate: 'Regenerating the token invalidates the previous embed URL. Continue?',
        close: 'Close'
      },
      errors: {
        network: 'Network request failed. Check the connection and try again.',
        request: 'The request parameters are invalid. Refresh and try again.',
        invalidSourceOrigin: 'The source origin must exactly match the Sub2API site connected to this workspace.',
        upstreamUnsupported: 'This Sub2API version does not support the leaderboard endpoint or token sorting. Upgrade it first.',
        unknown: 'The leaderboard request failed. Try again later.'
      }
    },
    lottery: {
      eyebrow: 'Lottery operations',
      title: 'Lottery Campaigns',
      subtitle: 'Create and operate Sub2API lottery campaigns, entries, draw results, reward delivery, audit trail, and embed token settings.',
      common: {
        empty: 'None',
        noDescription: 'No description provided.',
        yes: 'Yes',
        no: 'No'
      },
      filters: {
        status: 'Campaign status',
        all: 'All statuses'
      },
      tabs: {
        overview: 'Overview',
        entries: 'Entries',
        rewards: 'Winners & rewards',
        audit: 'Audit',
        embed: 'Embed'
      },
      list: {
        title: 'Campaigns',
        count: '{count} items',
        empty: 'No lottery campaigns match the current filter.'
      },
      detail: {
        empty: 'Select a campaign to view details.'
      },
      sections: {
        schedule: 'Schedule',
        integrity: 'Draw integrity',
        prizes: 'Prizes'
      },
      metrics: {
        entries: '{count} entries',
        winners: '{count} winners'
      },
      actions: {
        create: 'Create campaign',
        refresh: 'Refresh campaigns',
        edit: 'Edit draft',
        publish: 'Publish',
        close: 'Close registration',
        draw: 'Draw winners',
        cancel: 'Cancel campaign',
        retry: 'Retry',
        completeManual: 'Mark redeemed',
        save: 'Save campaign',
        closeDialog: 'Close',
        confirm: {
          publish: 'Publish this lottery campaign?',
          close: 'Close registration for this campaign?',
          draw: 'Draw winners for this campaign?',
          cancel: 'Cancel this campaign?',
          completeManual: 'Confirm that the winner completed manual redemption and mark this reward as fulfilled?'
        }
      },
      status: {
        draft: 'Draft',
        scheduled: 'Scheduled',
        open: 'Open',
        closed: 'Registration closed',
        drawing: 'Drawing',
        drawn: 'Drawn',
        fulfilling: 'Fulfilling',
        completed: 'Completed',
        partial: 'Partial',
        cancelled: 'Cancelled'
      },
      drawMode: {
        manual: 'Manual draw',
        scheduled: 'Scheduled draw'
      },
      prizeType: {
        balance: 'Balance',
        subscription: 'Subscription'
      },
      entryStatus: {
        active: 'Active',
        withdrawn: 'Withdrawn'
      },
      rewardStatus: {
        pending: 'Pending',
        processing: 'Processing',
        fulfilled: 'Fulfilled',
        retryable_failed: 'Retryable failed',
        manual_attention: 'Manual attention',
        failed: 'Failed'
      },
      fields: {
        actions: 'Actions',
        balanceAmount: 'Balance amount',
        createdAt: 'Created at',
        description: 'Description',
        drawAt: 'Draw at',
        drawMode: 'Draw mode',
        deliveryMode: 'Delivery method',
        entrySnapshotHash: 'Entry snapshot hash',
        entryId: 'Entry ID',
        error: 'Error',
        groupId: 'Group ID',
        groupName: 'Group name',
        subscriptionGroup: 'Subscription group',
        currentMultiplier: 'Current multiplier',
        maskedEmail: 'Masked email',
        multiplier: 'Multiplier',
        rewardMultiplier: 'Reward multiplier',
        name: 'Campaign name',
        prize: 'Prize',
        prizeName: 'Prize name',
        prizeSlot: 'Prize slot',
        prizeType: 'Prize type',
        publicWinners: 'Show winners publicly',
        quantity: 'Quantity',
        voucherCodes: 'Vouchers (one per line)',
        manualContact: 'Manual redemption contact',
        receiptHash: 'Receipt hash',
        registrationEnd: 'Registration end',
        registrationStart: 'Registration start',
        revealedSeed: 'Revealed seed',
        rewardStatus: 'Reward status',
        seedCommitment: 'Seed commitment',
        status: 'Status',
        validityDays: 'Validity days',
        winner: 'Winner'
      },
      entries: {
        empty: 'No entries yet.'
      },
      delivery: {
        title: 'Balance prize delivery',
        subtitle: 'Deliver each prize automatically, assign one voucher per winner, or provide a manual redemption contact.',
        quantityHint: '{count} units in this prize',
        mode: {
          sub2api_auto: 'Automatic',
          voucher: 'Vouchers',
          manual: 'Manual'
        },
        voucherPlaceholder: 'Enter one unique voucher code per line',
        voucherCount: '{current} provided; {required} required. Codes are assigned to winners in order after the draw.',
        manualPlaceholder: 'Enter an email, support account, or redemption instructions',
        manualHint: 'Only the winner and administrators can see this contact information.',
        autoHint: 'The current Sub2API workspace delivers the balance automatically after the draw.'
      },
      rewards: {
        empty: 'No reward jobs yet.',
        manualTitle: 'Manual redemptions awaiting confirmation'
      },
      prizes: {
        subscriptionSummary: '{group} (ID {id}) · Reward multiplier {multiplier} · {days} days'
      },
      audit: {
        create: 'Created',
        update: 'Updated',
        publish: 'Published',
        close: 'Closed',
        draw: 'Drawn',
        empty: 'No audit events yet.'
      },
      embed: {
        title: 'Lottery embed settings',
        subtitle: 'The current workspace is bound automatically. Use the URL in a Sub2API custom menu when the public page is enabled.',
        sourceOrigin: 'Bound site',
        url: 'Iframe URL',
        copy: 'Copy URL',
        copyFailed: 'Copy failed. Select the URL manually.',
        rotate: 'Regenerate token',
        confirmRotate: 'Regenerating the token invalidates the previous lottery embed URL. Continue?'
      },
      form: {
        createTitle: 'Create lottery campaign',
        editTitle: 'Edit lottery campaign',
        subtitle: 'Drafts can be edited before publishing. Public results are on by default, and balance prizes support automatic, voucher, or manual delivery.',
        namePlaceholder: 'July user lottery',
        descriptionPlaceholder: 'Internal campaign notes and public description.',
        addPrize: 'Add prize',
        removePrize: 'Remove',
        prizeNumber: 'Prize {number}',
        prizeNamePlaceholder: 'Prize display name',
        subscriptionGroupPlaceholder: 'Select a group from the current workspace',
        subscriptionGroupsLoading: 'Loading current workspace groups...',
        subscriptionGroupsEmpty: 'No available groups in this workspace',
        subscriptionGroupOption: '{name} (ID {id}) · Current multiplier {multiplier}',
        subscriptionGroupUnavailable: '{name} (ID {id}) · Saved reward multiplier {multiplier} · Currently unavailable',
        refreshSubscriptionGroups: 'Reload'
      },
      errors: {
        network: 'Network request failed. Check your connection and try again.',
        request: 'The lottery request is invalid. Refresh and try again.',
        unknown: 'The lottery request failed. Try again later.',
        invalidSourceOrigin: 'The lottery embed must use the current workspace Sub2API site.',
        notFound: 'The lottery campaign was not found.',
        invalidState: 'This lifecycle action is not valid for the current campaign status.',
        validation: 'Check required fields, schedule order, and prize configuration.',
        voucherQuantity: 'Provide one unique voucher per line and exactly match the prize quantity.',
        manualContactRequired: 'Manual redemption requires contact information the winner can use.',
        manualRedemptionRequired: 'The winner must use the displayed contact information to redeem this prize.',
        rewardSafeMessage: 'Reward delivery needs attention. Check the reward status and retry when available.',
        rewardUnsupported: 'The current Sub2API site does not support this reward type.',
        rewardAdminSession: 'Reconnect the current workspace Sub2API administrator account.',
        subscriptionGroups: 'Unable to load subscription groups for this workspace. Check that the Sub2API administrator session is still valid.'
      }
    },
    adminAccounts: {
      title: 'Select Workspace',
      subtitle: 'Choose an admin workspace to continue, or add a new one.',
      empty: 'No workspaces yet. Add your first workspace to get started.',
      currentLabel: 'Current workspace',
      addWorkspace: 'Add Workspace',
      addWorkspaceHint: 'Connect a new site admin account',
      creating: 'Creating workspace...',
      delete: {
        actionLabel: 'Delete workspace {name}',
        title: 'Delete {name}',
        localDataWarning: 'All 监控探测 local workspace data for this workspace will be permanently deleted. This cannot be undone.',
        remoteResourcesRetained: 'Remote upstream resources and accounts are retained and will not be deleted.',
        phraseInstruction: 'Type the exact phrase below to confirm: {phrase}',
        inputLabel: 'Confirmation phrase',
        inputPlaceholder: 'Type the phrase manually',
        cancel: 'Cancel',
        confirm: 'Delete Workspace',
        deleting: 'Deleting...',
        cleanupPending: 'Workspace deletion completed, but local runtime, cache, or file cleanup is still pending and will retry.'
      },
      errors: {
        noCurrentAccount: 'Please select a workspace first.',
        notFound: 'Workspace not found.',
        deleteFailed: 'Workspace deletion failed. Type the confirmation phrase again and retry.',
        request: 'Operation failed. Please try again.',
        network: 'Network error. Check your connection and try again.'
      }
    },
    dashboard: {
      metrics: {
        todayProfit: "Today's Revenue",
        siteBalance: 'Site User Balance',
        todayPurchase: "Today's Cost",
        netProfit: "Today's Net Profit",
        upstreamBalance: 'Upstream Total Balance',
        profitMargin: "Today's Margin",
        groupCount: 'My Groups',
        groupCountCaption: 'Click to view group details'
      },
      charts: {
        title: 'Trend Analytics',
        subtitle: 'Track continuous revenue, site user balance, cost, net profit and upstream balance over time.',
        trendTitle: '{metric} Trend'
      },
      period: {
        label: 'Period',
        week: 'Week',
        month: 'Month'
      },
      delta: {
        vsPrev: 'vs prev day',
        percentagePoints: '{value} pts'
      },
      common: {
        unavailable: 'N/A'
      },
      performance: {
        title: 'Business Performance',
        subtitle: 'Revenue, cost, and net profit over the same period',
        periodRevenue: 'Period Revenue',
        periodCost: 'Period Cost',
        periodProfit: 'Period Net Profit',
        chartAria: 'Combined revenue, cost, and net profit trend chart'
      },
      capital: {
        title: 'Capital Safety',
        subtitle: 'Balance coverage and cost runway',
        siteBalance: 'Site User Balance',
        upstreamBalance: 'Upstream Balance',
        coverage: 'Balance Coverage',
        coverageHint: 'Upstream balance relative to total site user balance',
        runway: 'Estimated Runway',
        runwayHint: 'Based on average daily cost for this period',
        runwayValue: '{value} days'
      },
      groups: {
        title: 'Group Contribution',
        subtitle: 'Top groups by revenue today',
        total: '{count} groups',
        amount: "Today's Revenue",
        topThreeShare: 'Top 3 revenue share',
        empty: 'No group revenue data available.',
        loadError: 'Group contribution data is temporarily unavailable.',
        chartAria: 'Today group revenue contribution ranking chart'
      },
      attention: {
        title: 'Needs Attention',
        subtitle: 'Health status and upstream balance issues',
        healthTitle: 'Target Health Issues',
        healthDescription: '{attention} need attention, {suspended} suspended or disabled',
        failuresTitle: 'Probe Failures in 24 Hours',
        failuresDescription: 'Review failure categories and recent state changes',
        upstreamTitle: 'Upstream Balance Issues',
        upstreamDescription: 'Some upstream sites have unknown balances or connection errors',
        allClearTitle: 'No Clear Issues',
        allClearDescription: 'No health target or upstream balance issue currently requires attention.',
        unavailableTitle: 'Operational Status Unavailable',
        unavailableDescription: 'Core business metrics are unaffected. Retry health and upstream status shortly.',
        lastProbe: 'Last probe: {time}',
        neverProbed: 'No records',
        refresh: 'Refresh operational status',
        partialLoadError: 'Some operational data failed to load'
      },
      loading: 'Loading metrics...',
      loadError: 'Failed to load dashboard metrics.',
      retry: 'Retry',
      dataStatus: {
        refreshing: 'Refreshing latest data',
        updatedAt: 'Updated at {time}',
        waiting: 'Waiting for first update',
        failed: 'Refresh failed; showing previous data',
        refresh: 'Refresh dashboard data'
      },
      loadingModal: {
        title: 'Loading Dashboard Data',
        progress: '{progress}% complete',
        steps: {
          auth: 'Verifying admin credentials...',
          data: 'Loading metrics and trends...',
          done: 'Preparing data and rendering page...'
        }
      },
      groupUsage: {
        title: 'Today\'s Revenue by Group',
        subtitle: '{count} groups, {total} total',
        close: 'Close',
        empty: 'No group usage data available.',
        loadError: 'Failed to load group usage data.',
        retry: 'Retry',
        columns: {
          groupName: 'Group Name',
          amount: 'Today\'s Amount'
        },
        sort: {
          desc: 'Amount: High to Low',
          asc: 'Amount: Low to High'
        }
      },
      upstreamKeyUsage: {
        title: 'Today\'s Cost Breakdown',
        subtitle: '{count} keys, {total} total',
        close: 'Close',
        empty: 'No keys with usage today.',
        loadError: 'Failed to load today\'s cost breakdown. Check upstream site connections and retry.',
        partialWarning: '{failed} of {total} upstream sites are unavailable. Totals include successful sites only.',
        retry: 'Retry',
        errors: {
          unavailable: 'Key usage is unavailable for every upstream site. Check site connections or credentials.'
        },
        columns: {
          siteName: 'Upstream Site',
          keyName: 'Key Name',
          groupName: 'Group',
          amount: 'Today\'s Amount'
        },
        sort: {
          desc: 'Amount: High to Low',
          asc: 'Amount: Low to High'
        }
      },
      upstreamBalanceBreakdown: {
        title: 'Upstream Balance Breakdown',
        subtitle: '{count} sites, {total} total',
        close: 'Close',
        empty: 'No upstream site balance data available.',
        loadError: 'Failed to load upstream balance breakdown.',
        retry: 'Retry',
        unknownBalance: 'Unknown',
        neverSynced: 'Never synced',
        columns: {
          siteName: 'Upstream Site',
          status: 'Status',
          lastSyncedAt: 'Last Synced',
          balance: 'Balance'
        },
        sort: {
          desc: 'Balance: High to Low',
          asc: 'Balance: Low to High'
        }
      },
      balanceFilter: {
        title: 'Balance Filter',
        subtitle: 'Configure filtering rules for site user balance calculation.',
        close: 'Close',
        excludeAdmin: 'Exclude admin accounts',
        excludeAdminHelp: 'Do not include admin role users in the balance total.',
        excludeBalances: 'Exclude specific balance values',
        excludeBalancesHelp: 'Users whose balance equals any of these values will be excluded.',
        addPlaceholder: 'Enter a balance value to exclude',
        add: 'Add',
        cancel: 'Cancel',
        save: 'Save',
        saving: 'Saving...',
        loadError: 'Failed to load filter config.',
        saveError: 'Failed to save filter config.'
      },
      adminAuth: {
        loggedInAs: 'Current admin: {identity}',
        logout: 'Sign out current admin',
        notLoggedIn: 'No admin account signed in',
        login: 'Sign in admin account',
        expiresAt: 'Expires',
        timeUnknown: 'Unknown',
        updateCredentials: 'Update Admin Credentials',
        updatingCredentials: 'Updating...',
        logoutConfirm: {
          title: 'Sign out current admin?',
          description: 'After signing out you must sign in and pass admin verification again to view dashboard data.',
          confirm: 'Sign out',
          cancel: 'Cancel'
        },
        dataLocked: {
          title: 'Sign in an admin account first',
          description: 'Dashboard stats are only visible after signing in and verifying a site account with admin permission.'
        },
        modal: {
          title: 'Sign in admin account',
          subtitle: 'The dashboard requires a site account with admin permission.',
          close: 'Close',
          platformLabel: 'Platform',
          platform: {
            sub2api: 'Sub2API',
            newapi: 'New-API'
          },
          comingSoon: 'Coming soon',
          newApiPasswordOnly: 'New-API supports username & password login only.',
          siteUrlLabel: 'Site address (domain or IP)',
          siteUrlPlaceholder: 'e.g. https://sub.example.com or http://1.2.3.4:5555',
          methodLabel: 'Login method',
          method: {
            password: 'Email & Password',
            token: 'RT + AT',
            admin_key: 'Admin Key'
          },
          fields: {
            emailPlaceholder: 'Admin email',
            usernamePlaceholder: 'Admin username',
            passwordPlaceholder: 'Admin password',
            accessTokenPlaceholder: 'Access Token (optional, can be empty)',
            refreshTokenPlaceholder: 'Refresh Token (enter this or an Access Token)',
            tokenTypePlaceholder: 'Token Type (optional, default Bearer)',
            tokenHelp: 'Enter an Access Token or Refresh Token. When a Refresh Token is present, near-expiry credentials refresh automatically.',
            sub2apiAdminKeyPlaceholder: 'Sub2API Admin API Key',
            sub2apiAdminKeyHelp: 'This key is sent only to Sub2API admin routes through x-api-key.',
            newApiAdminKeyPlaceholder: 'New-API system access token',
            newApiAdminKeyHelp: 'Use the system access token generated by the admin or Root account in personal settings.',
            userIdPlaceholder: 'User ID that owns this token'
          },
          submit: 'Sign in & verify',
          submitting: 'Verifying...'
        },
        errors: {
          request: 'Admin login request failed. Try again later.',
          missingCredentials: 'Fill in the site address and required fields for the selected method.',
          invalidUrl: 'Invalid site address. Enter a valid domain or IP and retry.',
          adminOnly: 'This account is not admin or verification failed. Check the credentials and retry.',
          network: 'Network or CORS request failed. Check the site URL.',
          platformUnsupported: 'Unsupported platform. Please choose Sub2API or New-API.',
          unknown: 'An unknown error occurred during admin login.',
          reloginRequired: 'Admin verification failed. Please sign in again.'
        }
      }
    },
    groupAssociations: {
      title: 'Pricing Mappings',
      detailsLabel: 'Pricing mapping details',
      subtitle: '{count} groups · {associated} configured · {unassociated} not configured',
      common: {
        placeholder: '—',
        multiplier: '{value}x',
        unknown: 'Unknown platform'
      },
      actions: {
        refresh: 'Refresh', retry: 'Retry', cleanup: 'Clean up stale config', editTargets: 'Edit sources', manage: 'Manage pricing mapping', openProfitCalculator: 'Open profit budget calculator', customProfitCalculator: 'Custom Profit Calculator'
      },
      filters: {
        searchLabel: 'Search pricing mappings', searchPlaceholder: 'Search own groups or upstream sources...',
        all: 'All', associated: 'Configured', unassociated: 'Not configured', stale: 'Stale'
      },
      listAria: 'My group list',
      targetCount: '{count} pricing sources',
      detailSubtitle: '{count} upstream groups used as pricing sources',
      staleOwnGroup: 'The admin site no longer returns this group. Its configuration is preserved until you confirm cleanup.',
      staleTarget: 'Upstream stale',
      metrics: {
        ownMultiplier: 'My group multiplier',
        targets: 'Pricing sources',
        budgetMargin: 'Budget gross margin',
        marginRange: '{minimum} - {maximum}',
        autoPricing: 'Auto-pricing',
        effectiveUpstream: 'Effective upstream multiplier',
        effectiveCost: 'Converted cost multiplier'
      },
      profitCalculator: {
        titleWithGroup: '{group} · Profit Budget',
        customTitle: 'Custom Profit Calculator',
        close: 'Close profit budget',
        modeLabel: 'Profit calculation mode',
        groupMode: 'Current Group',
        customMode: 'Custom Calculation',
        revenueLabel: 'Simulated Sales Revenue (CNY)',
        revenuePlaceholder: 'Enter simulated sales revenue',
        invalidRevenue: 'Enter a valid amount greater than or equal to 0.',
        ownMultiplier: 'My Selling Multiplier',
        upstreamCostMultiplier: 'Converted Upstream Cost Multiplier',
        saleMultiplier: 'My Selling Multiplier',
        multiplierPlaceholder: 'Enter multiplier',
        invalidUpstreamMultiplier: 'Enter a valid multiplier greater than or equal to 0.',
        invalidSaleMultiplier: 'Enter a valid multiplier greater than 0.',
        profitRange: 'Estimated Gross Profit Range',
        profitMargin: 'Estimated Profit Margin',
        amountRange: '{minimum} - {maximum}',
        estimatedCost: 'Estimated Purchase Cost',
        estimatedProfit: 'Estimated Gross Profit',
        noTargetsTitle: 'No Calculable Upstreams',
        noTargetsDescription: 'Configure a valid pricing source for this group first.'
      },
      sections: {
        targets: 'Pricing sources', targetsSummary: '{count} upstream sources', autoPricing: 'Auto-pricing policy'
      },
      noTargets: {
        title: 'No pricing sources', description: 'Add an upstream source before configuring auto-pricing.'
      },
      targetsDrawer: {
        titleWithGroup: '{group} · Edit pricing sources', selectedCount: '{count} upstream groups selected',
        searchLabel: 'Search upstream groups', searchPlaceholder: 'Search site, platform, or group...',
        emptyTitle: 'No matching upstream groups', emptyDescription: 'Adjust the search or sync an upstream site first.',
        unknownMultiplier: 'No multiplier', autoMultiplier: 'Auto', multiplier: '{value}x', stale: 'Stale',
        close: 'Close source editor', cancel: 'Cancel', save: 'Save sources', saving: 'Saving...'
      },
      cleanup: {
        title: 'Clean up stale configuration',
        description: 'This removes pricing sources and auto-pricing settings for “{group}”. It does not delete the remote group.',
        cancel: 'Cancel', confirm: 'Confirm cleanup'
      },
      errors: {
        primaryTargetRequired: 'The current primary upstream is used by auto-pricing. Change or disable auto-pricing before removing it.'
      },
      close: 'Close',
      empty: 'No group mappings found.',
      loadError: 'Failed to load group list.',
      runError: 'Failed to run auto-pricing. Please try again.',
      unassociatedLabel: 'No pricing source',
      unassociatedMultiplier: 'Not available',
      columns: {
        index: '#',
        ownGroup: 'Own Group',
        platform: 'Platform',
        groupType: 'Group Type',
        status: 'Status',
        ownMultiplier: 'Own Multiplier',
        upstreamGroup: 'Upstream Group',
        upstreamMultiplier: 'Upstream Multiplier',
        autoPricing: 'Auto Pricing'
      },
      exclusiveLabels: {
        public: 'Public',
        exclusive: 'Exclusive'
      },
      statusLabels: {
        active: 'Active',
        inactive: 'Inactive'
      },
      autoPricingTip: 'When enabled, automatically adds a markup on top of the upstream multiplier during sync. Supports fixed value or percentage strategies.',
      autoPricingStatus: {
        notConfigured: 'Not configured',
        enabled: 'Enabled',
        savedDisabled: 'Saved, not enabled'
      },
      autoPricingActions: {
        configure: 'Configure',
        edit: 'Edit',
        runNow: 'Run Now',
        runNowFor: 'Run auto-pricing now for {group}'
      },
      lastRun: {
        never: 'Never',
        summary: 'Last: {status} · {trigger} · {time}',
        reason: 'Reason: {reason}',
        triggerManual: 'Manual',
        triggerAfterSync: 'After sync',
        triggerUnknown: 'Unknown trigger',
        reasonUnknown: 'No details available',
        status: {
          applied: 'Success',
          skipped: 'Skipped',
          thresholdExceeded: 'Threshold exceeded',
          failed: 'Failed',
          unknown: 'No run'
        },
        reasons: {
          primary_upstream_not_affected: 'Primary upstream was not affected by the sync.',
          missing_reference_multiplier: 'Reference multiplier is missing.',
          unknown_pricing_source: 'Pricing source is not recognized.',
          status_persist_failed: 'Run status could not be saved.',
          invalid_old_reference_multiplier: 'Previous reference multiplier is invalid.',
          threshold_exceeded: 'Change exceeded the configured threshold.',
          own_group_not_found_in_admin: 'Own group was not found in the admin site.',
          target_unchanged: 'Target multiplier was unchanged.',
          remote_update_failed: 'Remote multiplier update failed.'
        }
      },
      autoPricingDrawer: {
        title: 'Auto-Pricing Config',
        titleWithGroup: '{group} · Auto-Pricing Config',
        enableLabel: 'Enable Auto-Pricing',
        sourceLabel: 'Pricing Source',
        sourcePrimaryUpstream: 'Primary Upstream',
        sourceLowestUpstream: 'Lowest Upstream',
        sourceHighestUpstream: 'Highest Upstream',
        sourceAverageUpstream: 'Average Upstream',
        primaryUpstreamLabel: 'Primary Upstream',
        primaryUpstreamPlaceholder: 'Select primary upstream',
        strategyLabel: 'Markup Method',
        strategyFixed: 'Fixed Increase',
        strategyPercentage: 'Percentage Increase',
        fixedIncreaseLabel: 'Fixed Increase Value',
        percentageIncreaseLabel: 'Percentage Increase Value',
        thresholdLabel: 'Follow Threshold',
        thresholdHelp: 'Auto-follow only when upstream change is within this percentage',
        minMultiplierLabel: 'Min Multiplier',
        maxMultiplierLabel: 'Max Multiplier',
        estimatedMultiplier: 'Estimated Multiplier',
        save: 'Save Config',
        cancel: 'Cancel',
        noUpstreams: 'No upstreams linked to this group. Cannot configure auto-pricing.',
        noMultiplierData: 'No upstream multiplier data available. Cannot compute estimated multiplier.',
        tips: {
          minMultiplier: 'The calculated multiplier will not go below this value. Use it to protect your minimum margin. Leave empty for no lower limit.',
          maxMultiplier: 'The calculated multiplier will not go above this value. Use it to avoid sudden price spikes for users. Leave empty for no upper limit.',
          threshold: 'Auto-follow only when the upstream multiplier changes within this percentage. Larger changes should wait for manual confirmation to avoid abnormal upstream swings changing your group price.',
          minMultiplierAria: 'View min multiplier help',
          maxMultiplierAria: 'View max multiplier help',
          thresholdAria: 'View follow threshold help',
        },
        guidance: {
          title: 'Recommended Settings',
          minMultiplier: 'Min Multiplier: your cost + minimum profit margin',
          maxMultiplier: 'Max Multiplier: the highest price users would still accept',
          threshold: 'Follow Threshold: 10%',
          exampleTitle: 'Calculation Example',
          exampleOld: 'Upstream old multiplier: 1.00',
          exampleNew: 'Upstream new multiplier: 1.08',
          exampleThreshold: 'Follow threshold: 10%',
          exampleMarkup: 'Markup method: upstream + 0.10',
          exampleMin: 'Min multiplier: 1.00',
          exampleMax: 'Max multiplier: 1.30',
          exampleResult: 'The change is 8%, within the 10% threshold, so auto-follow is allowed. The final multiplier is 1.18, which falls within the 1.00–1.30 limit range.',
        },
        notify: {
          sectionTitle: 'Auto-Pricing Success Notification',
          enableLabel: 'Send notification after pricing update',
          enableHelp: 'Send a bot notification when auto-pricing actually updates the group multiplier.',
          botSelectLabel: 'Notification Bots',
          botSelectPlaceholder: 'Select bots to notify',
          noBots: 'No bots available. Please configure bots in System Settings > Notifications & Channels first.',
          templateLabel: 'Notification Template',
          templateHelp: 'Leave empty to use the default template. Supported variables:',
          templatePlaceholder: 'Leave empty to use default template',
          defaultTemplate: '[Auto Pricing] {ownGroup} was adjusted from {oldOwnMultiplier}x to {newOwnMultiplier}x. Reference: {upstreamSiteName} / {upstreamGroupName}, multiplier {oldReference}x -> {newReference}x.',
          variablesTitle: 'Available Variables',
          varOwnGroup: 'Own group name',
          varUpstreamSiteName: 'Upstream site name',
          varUpstreamGroupName: 'Upstream group name / source',
          varOldReference: 'Old reference multiplier',
          varNewReference: 'New reference multiplier',
          varOldOwnMultiplier: 'Multiplier before adjustment',
          varNewOwnMultiplier: 'Multiplier after adjustment',
          varStrategy: 'Pricing strategy',
          varFixedIncrease: 'Fixed increase value',
          varPercentageIncrease: 'Percentage increase value',
          varThreshold: 'Follow threshold',
          copied: 'Copied',
        },
        errors: {
          primaryRequired: 'Primary upstream must be selected in primary upstream mode.',
          increaseNonNegative: 'Increase value cannot be negative.',
          thresholdNonNegative: 'Threshold cannot be negative.',
          multiplierNonNegative: 'Multiplier cannot be negative.',
          minGreaterThanMax: 'Min multiplier cannot be greater than max multiplier.',
          invalidConfig: 'Invalid auto-pricing config. Please check and try again.',
          notifyBotsRequired: 'At least one bot must be selected when notifications are enabled.',
        }
      },
      save: 'Save',
      saveSuccess: 'Saved',
      saving: 'Saving...',
      saveError: 'Save failed. Please try again.'
    },
    connectionHealth: {
      channelPriority: {
        label: 'Allow priority changes',
        toggle: 'Allow automatic priority changes for {name}',
        hint: 'Enabled by default, following group policies. Turning off restores the original priority and stops automatic adjustments. Manual upstream edits are preserved. Shared across groups; probing and suspension are controlled independently.',
        off: 'Automatic priority off',
        restoring: 'Automatic priority is off. Restoring the original priority; retries are automatic.'
      },
      channelSuspension: {
        label: 'Allow suspension',
        toggle: 'Allow automatic suspension of {name}',
        hint: 'Off: no automatic suspension; probing and the priority switch are unaffected. Applies across all groups sharing this channel. System-suspended channels are restored; manual disables are preserved. On still requires policy permission.'
      },
      probeBudget: {
        exhausted: '{models} · Daily probe limit reached {used}/{limit}. Resumes {time} (UTC+8)',
        unavailable: 'Probe quota could not be loaded. Please refresh.'
      },
      latencyPriority: {
        averageFirstToken: 'Weighted first token',
  "help": "Only updates upstream priority. Average recent successful policy probes (including manual probes) of one selected model.",
  "model": "Model for latency ranking",
  "firstModel": "First enabled model (alphabetical)",
  "sampleCount": "Sample count",
  "maxAge": "Sample validity (seconds)",
  "minSamples": "Minimum samples",
  "hysteresis": "Band margin (seconds)",
  "windowHelp": "Defaults for 60-second probes: 5 successful samples within 300 seconds (5 minutes), weighted 30/25/20/15/10 from newest to oldest. Available weights are normalized when fewer samples exist; below the minimum uses the insufficient-data priority. Set the band margin to 0 to disable it.",
  "weights": "Weights, newest to oldest (normalized automatically)",
  "weightItem": "Sample {n}",
  "bandsHelp": "Contiguous ranges must start at 0, with no overlaps. The boundary belongs to the preceding band; leave the final upper bound empty. Lower numbers are preferred by Sub2API; NewAPI prefers higher numbers.",
  "from": "From (s)",
  "to": "Through (s)",
  "unbounded": "No limit",
  "priority": "Priority",
  "removeBand": "Remove band {n}",
  "addBand": "Add band",
  "insufficientPriority": "Insufficient data",
  "degradedPriority": "Degraded priority",
  "suspendedPriority": "Suspended / disabled",
  "sharedHelp": "A shared account has one upstream priority. The least preferred result wins across active latency policies. Health transitions still follow failure/recovery thresholds.",
  "average": "Weighted latency",
  "samples": "{used}/{total} successes",
  "decision": "Calculated priority {priority}",
  "details": "Calculation details",
  "modelLine": "Model: {model} · Validity: {seconds}s",
  "policyLine": "Policy: {name}",
  "shared": "Shared account: {count} latency policies",
  "sampleLine": "{time} · {seconds}s · Weight {weight}%",
  "reasons": {
    "latency": "Latency band {n}",
    "insufficient": "Insufficient fresh samples",
    "degraded": "Failure threshold reached",
    "suspended": "Suspended / disabled"
  },
  "unavailable": "Latency calculation unavailable",
  "conflict": "Manually changed upstream; automatic updates stopped",
  "partial": "The average is informational until enough samples exist"
},
      groupAutomation: {
        groupLabel: 'Automation for {group}',
        none: 'No policy',
        configure: 'Configure automation for {group}',
        edit: 'Edit automation for {group}',
        editPolicy: 'Edit policy {policy}',
        manage: 'Expand {count} policies for {group}',
        enable: 'Enable policy "{policy}" for {group}',
        disable: 'Disable policy "{policy}" for {group}',
        enabled: 'Policy enabled',
        disabled: 'Policy disabled',
        unavailable: 'Policy unavailable',
        unavailableHint: 'Policy details could not be loaded. Refresh and try again.',
        multipleHint: '{count} policies, {enabled} enabled. Expand to edit or toggle each one.',
        shared: 'This policy is used by {count} groups. Edits and toggles apply to all of them.',
        assignments: 'Manage policies and channels',
        modes: { monitor: 'Probe only', priority: 'Auto priority', priorityOnly: 'Priority only', suspend: 'Can suspend', localSuspend: 'Local suspension', unconfigured: 'Needs models', mixed: 'Mixed policies' },
        hints: {
          monitor: 'Probe and record health without changing upstream priority, status or weight.',
          priority: 'Probe and adjust upstream priority using multiplier and health settings, without disabling channels.',
          priorityOnly: 'Adjust priority by group multiplier without automatic channel probes.',
          suspend: 'Probe results may disable and restore upstream channels. Priority changes depend on policy settings.',
          localSuspend: 'Local probe suspension is allowed, but remote actions are off: upstream channels stay enabled.',
          unconfigured: 'No probe model is enabled. Edit the policy to configure models.',
        },
      },
      quality: {
        detail: {
          button: 'Details', channelButton: 'Quality details', open: 'View quality check details for {name}', title: 'Quality checks · {name}',
          subtitle: 'Newest start time first, including custom questions, API Candy and API Pelican checks.',
          filter: 'Filter check type', allTypes: 'All check types', refresh: 'Refresh check history', records: 'Quality check history',
          empty: 'No checks of this type yet.', count: '{count} records', selectRecord: 'Select a record to view details',
          started: 'Started:', finished: 'Finished:', duration: 'Duration', assessment: 'API assessment',
          results: { passed: 'Passed', failed: 'Abnormal', error: 'Check failed' },
          preview: 'Preview artwork', source: 'View source', stopPreview: 'Stop preview', previewTitle: 'Pelican artwork preview', previewStopped: 'Preview stopped. Select Preview artwork to reload.',
          noHtml: 'No artwork HTML was saved for this record. Older artwork cannot be recovered; new checks retain the returned artwork.', htmlTooLarge: 'Artwork exceeded 2 MB and was not saved. The assessment is retained.',
          question: 'Question', noPrompt: 'The question text was not saved for this older record.', matchMode: 'Match mode:', judgement: 'Rules and results', noAnswer: 'No answer was returned.', candyHint: 'Candy verdicts come from the API and do not use local question answers or latency thresholds.',
        },
        automaticLabel: 'Auto quality', configureManualFirst: 'Save the global quality settings first. Automatic detection can remain off.',
        inactiveAutomationHint: 'Automatic health and quality checks are off without an enabled automation policy. Manual tests remain available.',
        detectionMethod: 'Detection method', methodQuestions: 'Custom questions', methodManxue: 'Manxue AI API',
        manxueOptions: 'API detection', manxueBenchmark: 'Benchmark', benchmarks: { candy: 'Candy test', pelican: 'Pelican test' },
        manxueProtocol: 'Upstream protocol', manxueServiceTier: 'Service tier', manxueDocs: 'API documentation',
        manxueDisclosure: 'Saving this method sends each tested channel’s public HTTPS URL, API key and model to manxue.ai, which calls the channel to run the test. The API is free; upstream model usage consumes channel credits. No separate Manxue API key is needed.',
        manxueVerdictHint: 'Uses the Manxue verdict. Passed candy / normal pelican: green. Incorrect / degraded: red. Errors or unknown verdicts: amber. Follows existing group and channel switches; consecutive abnormal results can trigger suspension. Local questions, duration thresholds and token limits are not used. Configure a task timeout of up to 600 seconds under Detection settings.',
        manxueInvalidConfig: 'Check Manxue options: candy requires Responses; pelican supports default, low, medium or high reasoning effort.',
        manxueUnsupported: 'Manxue requires an OpenAI-compatible channel; this native protocol is unsupported.',
        manxuePublicURL: 'Manxue requires a publicly accessible HTTPS channel URL.',
        manxueUnknown: 'Manxue returned no definite verdict. This does not count as degradation.',
        manxueFailed: 'Manxue could not complete the test. This does not count as degradation.',
        manxueExpired: 'The Manxue task was not found or has expired.',
        manxueTimeout: 'Manxue timed out; cancellation of this task was attempted.',
        toggleChannel: 'Toggle quality checks for channel {name}', channelSwitchHint: 'Follows the group by default. Explicitly enabling a channel allows automatic checks while group quality is off; disabling it overrides the group. Requires global checks and selection in an enabled policy. Shared across groups; manual tests and history are unaffected.',
        settingsTitle: 'Quality detection settings', settingsHint: 'These settings are shared across the workspace. Enable a group or an individual channel. Automatic checks still require selection in an enabled automation policy.',
        displayOnly: 'Results only. Channel status, priority and weight are unaffected.',
        autoSuspendEnabled: 'Suspend channels after quality degradation',
        actionSummary: 'Quality suspension is enabled, subject to group and channel suspension permissions.',
        suspensionRules: 'Suspend when consecutive red results reach the failure threshold. Amber request errors do not count as degradation. Continue checks according to automatic quality settings and restore after the recovery threshold; any health suspension still holds the channel paused. Manual checks also count. Requires auto-degrade, remote actions and suspension permission in the group policy, plus channel suspension permission.',
        actionFailed: 'Could not update channel status after the quality check.',
        globalEnabled: 'Enable quality detection', questions: 'Question checks', schedule: 'Detection settings', model: 'Detection model', reasoningEffort: 'Reasoning effort', defaultEffort: 'Model default', mode: 'Evaluation',
        modes: { content_time: 'Content and duration', content: 'Content only', time: 'Duration only' },
        questionBank: 'Question bank', add: 'Add question', newQuestion: 'New question', questionName: 'Question name', maxDurationMs: 'Duration threshold (ms)', prompt: 'Question prompt', matchMode: 'Matching',
        matchModes: { answer: 'Exact answer', keyword: 'Contains keyword', regex: 'Regular expression' }, answer: 'Answer / keyword / pattern', answerHint: 'Exact answers accept plain text or a single FINAL_ANSWER= line. Each valid response advances to the next enabled question.',
        intervalSeconds: 'Normal interval (seconds)', retrySeconds: 'Abnormal / failed retry interval (seconds)', failureLimit: 'Consecutive abnormal answers', recoveryLimit: 'Consecutive passes to recover', concurrency: 'Concurrency (1–32)', timeoutSeconds: 'Request timeout (seconds)', maxTokens: 'Maximum output tokens', historyLimit: 'History per channel (1–1000)',
        scheduleHint: 'Streaks determine degradation and recovery, and control channel suspension when enabled. Network errors, rate limits, empty and incomplete responses do not count as wrong answers. The strip shows up to 100 checks per channel. Changing settings or pausing keeps existing records; streaks restart for the new configuration.',
        importConfig: 'Import Qerkai questions', importHint: 'Import disables detection until you review, enable and save the settings.', importFailed: 'Choose a valid Qerkai JSON configuration under 1 MB.',
        save: 'Save settings', saving: 'Saving…', loading: 'Loading…', close: 'Close', retry: 'Retry', moveUp: 'Move question up', moveDown: 'Move question down', enableQuestion: 'Enable question {name}', removeQuestion: 'Delete question {name}',
        manualLabels: { questions: 'Custom questions', manxue_candy: 'API Candy', manxue_pelican: 'API Pelican' },
        healthPausedHint: 'Quality checks are paused while this channel is health-suspended. They resume with the saved settings after health recovery; existing history is kept.',
        manualHints: { questions: 'Run one check using the saved custom question bank.', manxue_candy: 'Run a Manxue API Candy test. Sends this channel’s URL, key and model to Manxue and consumes channel credits.', manxue_pelican: 'Run a Manxue API Pelican test. Sends this channel’s URL, key and model to Manxue and consumes channel credits.' },
        probingNow: 'Checking quality…', probeBusy: 'A quality check is already running for this channel. Refresh shortly to see its result.', probeUnavailable: 'This channel cannot be checked or its configuration has changed. Check the channel and refresh.',
        stripTitle: 'Quality checks', toggleGroup: 'Toggle quality detection for {name}', globalPaused: 'Globally paused', configureFirst: 'Enable and save global quality settings first, then enable groups on the left.',
        invalidModel: 'Enter a valid detection model.', invalidConfig: 'Check parameters: interval 10–86400s, streaks 1–20, concurrency 1–32, question timeout 5–300s / API timeout 5–600s, output tokens 128–32768, history 1–1000.', invalidQuestion: 'Enter a question name, prompt and answer; check matching mode and duration threshold (1–300000 ms).', invalidRegex: 'The answer regular expression is invalid.', questionsRequired: 'Enable at least one question. The bank supports up to 50 questions.',
        statuses: { qualitySuspended: 'Quality suspended', manualOnly: 'Manual tests only', healthPaused: 'Paused by health check', normal: 'Normal', suspect: 'Suspected degradation', degraded: 'Degraded', recovering: 'Recovering', error: 'Check failed', off: 'Disabled', notSelected: 'Not selected', pending: 'Pending', unavailable: 'History unavailable' },
        selectionHint: 'This channel is not selected in automation policies and will not be tested. Existing history is retained.',
        passed: 'Passed', failed: 'Abnormal answer', error: 'Check failed', recent: 'Last {count}', failedCount: 'Abnormal {count}', errorCount: 'Failed {count}', passRate: 'Pass rate {rate}', rateHint: 'Only valid answers count; request failures are excluded.',
        passRateShort: '{rate} passed', passedRatio: '{passed}/{total} passed',
        historyLabel: 'Last {count} quality checks: {failed} abnormal answers, {errors} failed requests, {rate} pass rate. Green points at the top pass, red points at the bottom are abnormal, and amber points in the middle are request errors. Oldest on the left. Use arrow keys to inspect results.',
        noRecord: 'No check recorded', actualAnswer: 'Actual answer', expectedAnswer: 'Expected answer / rule', contentResult: 'Content', timeResult: 'Duration', previousDegraded: 'Previous degradation verdict retained',
        historyUnavailable: 'Quality history is unavailable. Refresh to retry.', waiting: 'Waiting for background checks. Green: passed; red: abnormal answer; amber: check failed.', enableHint: 'Enable group and channel quality checks to start recording.',
      },
      groupProbe: {
        title: 'Automatic group probing',
        close: 'Close',
        hint: 'Save to probe automatically at the chosen interval, even with the page closed. A blank key keeps the saved key; a dedicated key is created and reused only if none is configured.',
        key: 'Group key (optional)',
        keyPlaceholder: 'Enter a group key, or leave blank to create one',
        keySavedPlaceholder: 'Leave blank to keep the saved key, or enter a replacement',
        keySaved: 'Group key saved',
        useAutoKey: 'Use an automatic key',
        fetchModels: 'Fetch model list',
        customKeyUnavailable: 'This key is unavailable. Check that it is active, bound to this group, and accessible with the connected administrator account.',
        interval: 'Probe interval (seconds)',
        intervalInvalid: 'Enter a whole number between 10 and 86400 seconds.',
        enabled: 'Enable automatic probing',
        save: 'Save settings',
        saving: 'Saving…',
        autoEvery: 'Auto · {seconds}s',
        paused: 'Paused',
        preparing: 'Loading…',
        model: 'Probe model',
        modelPlaceholder: 'Select or enter a model name',
        modelListHint: 'Models could not be listed. Enter a model supported by this group.',
        retry: 'Retry',
        probing: 'Probing…',
        start: 'Start probe',
        action: 'Configure automatic probes for {name}',
        buttonHint: 'Set the group probe model and interval',
        channelCount: '{total} channels · {count} automatically monitored',
        channels: 'Channel probe history',
        selectGroup: 'Select a group to view its channels',
        unsupported: 'Group probing currently supports Sub2API only.',
        loginRequired: 'Sign in to the connected Sub2API account first. An admin API key alone cannot create a group API key.',
        keyUnavailable: 'Could not prepare the group key. Check group permissions and the status and group binding of the existing probe key.',
        modelRequired: 'Enter a valid probe model name.'
      },
      cards: {
        loadingChannels: 'Loading channels…',
        accounts: 'Monitoring {count}/{total}',
        recent: 'Last {count} probes',
        slow: 'Slow {count}',
        errors: 'Errors {count}',
        successRate: 'Success {value}',
        averageFirstToken: 'Average first token',
        averageLatency: 'Avg. probe',
        latencyHint: 'Average successful duration for the latest probe method, within the latest 100 probes',
        configure: 'Configure monitoring for {name}',
        expand: 'Expand accounts in {name}',
        collapse: 'Collapse accounts in {name}',
        historyLabel: 'Latest {count} probes, {errors} errors, {rate} success. Oldest on the left, newest on the right.',
        historyUnavailable: 'Probe history is unavailable. Refresh to try again.',
        noHistory: 'No probe history yet. Results will appear after probing.',
        noRecord: 'No record',
        success: 'Success',
        failure: 'Failure',
        slowLegend: 'Slow ≥5s',
        legend: 'Probe history legend',
        justNow: 'Just now',
        minutesAgo: '{count} min ago',
        loading: 'Loading groups',
        noMatches: 'No matching groups',
        inactiveChannels: 'Channels without active automation',
        clearFilters: 'Clear filters',
        priorityConflict: '{count} channels have priority conflicts. Review the notices below.',
        status: {
          qualitySuspended: 'Quality suspended', healthy: 'Healthy',
          unhealthy: 'Unhealthy',
          attention: 'Observing',
          pending: 'Pending',
          unmonitored: 'Unmonitored',
          unavailable: 'Unavailable',
          paused: 'Paused',
          inactive: 'Group disabled',
          empty: 'No accounts',
          loadError: 'Load failed'
        }
      },
      title: 'Group Health',
      subtitle: 'Independent lightweight probing of accounts/channels inside the current admin workspace groups, with health monitoring and automatic degrade/restore.',
      adminSubtitle: 'Shows all groups under the current admin workspace. Click the account count to view accounts/channels and their independent probe status.',
      simplifiedSubtitle: 'Configure probes, automatic failover, and traffic priority around admin upstream groups. New accounts and channels inherit their group policy automatically.',
      summaryLabel: 'Group health summary',
      groupListLabel: 'Upstream group list',
      refresh: 'Refresh',
      empty: 'No probeable accounts/channels under the current admin workspace.',
      adminEmpty: 'No groups under the current admin workspace.',
      notConnected: 'Not connected',
      notProbed: 'Not probed yet',
      notConfigured: 'No probe model configured',
      groupTypes: {
        public: 'Public',
        exclusive: 'Exclusive',
        subscription: 'Subscription'
      },
      groupStatusLabels: {
        active: 'Active',
        inactive: 'Inactive'
      },
      adminColumns: {
        name: 'Name',
        platform: 'Platform',
        type: 'Type',
        multiplier: 'Multiplier',
        accounts: 'Accounts',
        accountsUnit: '',
        status: 'Group Status',
        probeOverview: 'Probe Overview',
        detail: 'Details'
      },
      adminOverview: {
        probeable: 'Probeable {probeable}/{total}',
        noneProbeable: 'No probeable targets',
        noProbe: '{count} pending probe'
      },
      groupList: {
        monitored: 'Monitored {count}/{total}'
      },
      groupDetail: {
        multiplierPriority: 'Multiplier priority',
        subtitle: '{monitored} of {total} accounts or channels monitored',
        enableMonitoring: 'Configure Group Strategy',
        manageMonitoring: 'Manage Group Strategy',
        unmonitored: 'Not auto-monitored',
        policyCount: '{name} and {count} more',
        unprobeable: 'Probe unavailable',
        upstreamStatus: 'Upstream status: {status}',
        unknownUpstreamStatus: 'Unknown',
        priorityConflict: '{count} upstream priorities were changed manually. Automatic priority management has stopped for those targets to preserve the manual values. Save the group policy again to take control.',
        priorityConflictShort: 'Upstream priority changed manually; automatic updates stopped',
        empty: 'This group has no accounts or channels.',
        metrics: {
          accounts: 'Accounts / Channels',
          monitored: 'Auto-monitored',
          probeable: 'Manual probe ready',
          lastProbe: 'Last Probe'
        },
        statusBreakdown: {
          title: 'Current Group Probe States',
          hint: 'Health states are counted by model; pending and unavailable are counted by target. These are not upstream enable/disable states.',
          healthy: 'Healthy Models',
          degraded: 'Degraded Models',
          suspended: 'Probe Paused',
          observing: 'Recovery Watch',
          recovering: 'Recovering',
          disabled: 'Manually Disabled',
          notProbed: 'Awaiting First Probe',
          unprobeable: 'Unavailable Targets'
        },
        assignmentSources: {
          none: 'No policy source',
          target: 'Target-specific policy',
          group: 'Inherited group policy',
          mixed: 'Group and target policies merged'
        },
        columns: {
          expand: 'Expand model results',
          account: 'Account / Channel',
          health: 'Health',
          strategy: 'Effective Policy',
          priority: 'Upstream Priority',
          multiplier: 'Effective Multiplier',
          strategyMultiplier: 'My Group Multiplier',
          upstreamMultiplier: 'Upstream API Key Multiplier',
          actions: 'Actions'
        },
        upstreamMultiplierPending: 'Shown after linking',
        models: {
          empty: 'This target has no model probe results yet.',
          latency: 'Latency {value} ms',
          lastProbe: 'Last {value}',
          weight: 'Health weight {value}%'
        }
      },
      setup: {
        title: 'Configure Group Automation',
        stepsLabel: 'Group automation setup steps',
        steps: {
          '1': 'Effective Scope',
          '2': 'Run Strategy',
          '3': 'Review'
        },
		generatedPolicyName: '{group} - Group Automation Policy',
		retry: 'Reload',
        scope: {
          title: 'Choose Strategy Targets',
          description: 'Every account or channel in this group is selected by default. Unchecked targets are not auto-probed, failed over, or reprioritized.',
          modelsUnknown: 'No model list returned by upstream',
          probeable: 'Probe ready',
          pending: 'Credentials needed',
          futureHint: 'Targets added to this upstream group later inherit the policy automatically. Targets you uncheck remain excluded.'
        },
        strategy: {
          title: 'Choose a run strategy',
          description: 'Create a probe strategy, a multiplier-only priority strategy, or bind an existing advanced policy.',
          options: {
            multiplier: {
              title: 'Latency Priority',
              description: 'Map the weighted latency of the latest 5 successful probes within 300 seconds to configured upstream priorities.'
            },
            multiplierOnly: {
              title: 'Multiplier Only',
              description: 'Adjust upstream priority only by group multiplier, without model probes, degradation, or remote health actions.'
            },
            stable: {
              title: 'Stability First',
              description: 'Auto-probe and disable or restore failed targets using platform capabilities without changing routine priority.'
            },
            monitor: {
              title: 'Monitor Only',
              description: 'Record health and events without upstream disable, restore, or priority changes.'
            },
            existing: {
              title: 'Use Existing Policies',
              description: 'Bind one or more advanced probe policies when you need custom thresholds and budgets.'
            }
          },
          modelsLabel: 'Probe Models',
          modelsPlaceholder: 'One model per line, for example gpt-4o-mini',
          modelsDetected: '{count} models will be probed',
          modelSuggestions: {
            common: '{count} models available on every selected target were recommended from existing data; no extra upstream request was made.',
            discovered: 'No complete intersection was found. {count} models were recommended from existing model data and can be edited.'
          },
          providerLabel: 'Model Provider',
          remoteActionLabel: 'Run Upstream Actions',
          remoteActionHelp: 'Automatically disable or restore targets on failure and recovery when the platform supports it',
          multiplierOnlyTitle: 'Sync Multiplier Priority Only',
          multiplierOnlyHelp: 'The scheduler reads current group multipliers about every 30 seconds. Lower multipliers receive higher priority. No models or probe credentials are required.',
          multiplierMissingTitle: 'This group has no valid multiplier',
          multiplierMissingHelp: 'Set the group multiplier upstream before enabling multiplier sorting. The system will not assume 1x or change the current priority.'
        },
        confirm: {
          title: 'Review Group Configuration',
          description: 'Saving creates the group-level policy relationship immediately. The scheduler applies it on its next scan.',
          scope: 'Effective Scope',
          scopeValue: '{selected} selected, {excluded} excluded',
          strategy: 'Run Strategy',
          models: 'Probe Models',
          fromPolicy: 'Defined by existing policies',
          notApplicable: 'Not required',
          remoteAction: 'Upstream Automation',
          enabled: 'Enabled',
          disabled: 'Disabled',
          multiplierRule: 'Latency rule: use the latest 5 successful probes within 300 seconds with weights 30/25/20/15/10, then map the average to a configured priority. Shared channels use the least preferred result. Failure and recovery thresholds still apply; manual priority edits stop automatic overwrites.',
          multiplierOnlyRule: 'Multiplier-only rule: health state is ignored and no model probes run. Targets in multiple groups use the lowest multiplier. Disabling or unbinding restores the original priority; manual edits remain protected by conflict detection.'
        },
        back: 'Back',
        next: 'Next',
        save: 'Save Group Strategy'
      },
      probeUnavailableReasons: {
        credential_unavailable: 'Cannot securely obtain upstream credentials; probing unavailable',
        secure_verification_required: 'Upstream root secure verification is required to read the channel key',
        base_url_unavailable: 'No usable Base URL; probing unavailable',
        model_unavailable: 'No probe model available (configure one in a probe policy)',
        export_unavailable: 'Upstream account export endpoint unavailable; cannot obtain credentials',
        credentials_redacted: 'Upstream credentials are redacted and cannot be used for probing'
      },
      accountsDialog: {
        multiplier: 'Multiplier',
        unknownPlatform: 'Unknown platform',
        unknownStatus: 'Unknown status',
        empty: 'No accounts/channels under this group.',
        noProbeData: 'No probe data',
        unprobeable: 'Not probeable',
        unassignedPolicy: 'No policy assigned',
        unassignedPolicyHint: 'No policy assigned — will not be auto-probed. Manual one-time probing is still available.',
        assignedPolicyCount: '{name} and {count} more',
        assignPolicy: 'Assign policy',
        columns: {
          name: 'Name',
          platform: 'Platform',
          type: 'Type',
          status: 'Status',
          priority: 'Priority',
          concurrency: 'Concurrency',
          weight: 'Weight',
          models: 'Models',
          probeStatus: 'Probe Status',
          policyAssignment: 'Policy Assignment',
          actions: 'Actions'
        }
      },
      summary: {
        total: 'Probe Targets',
        unconfigured: 'Not Configured',
        monitoredGroups: 'Monitored Groups',
        priorityConflicts: 'Priority Conflicts'
      },
      stateLabels: {
        healthy: 'Healthy',
        degraded: 'Degraded',
        suspended: 'Probe Paused',
        observing: 'Observing',
        recovering: 'Recovering',
        disabled: 'Disabled'
      },
      providerLabels: {
        gemini: 'Gemini',
        anthropic: 'Anthropic',
        openai: 'OpenAI',
        custom: 'Custom'
      },
      filters: {
        allGroups: 'All Own Groups',
        allSites: 'All Upstream Sites',
        allStates: 'All States',
        allProviders: 'All Model Types',
        searchGroup: 'Search group name...',
        allPlatforms: 'All Platforms',
        allTypes: 'All Types'
      },
      columns: {
        model: 'Model',
        state: 'State',
        weight: 'Weight',
        lastProbe: 'Last Probe',
        lastError: 'Last Error'
      },
      actions: {
        probe: 'Probe Now', probeShort: 'Probe',
        disable: 'Disable',
        restore: 'Restore',
        viewEvents: 'View Events'
      },
      errorKeys: {
        quality_suspend: 'Quality suspension', quality_restore: 'Quality recovery', quality_action_failed: 'Quality action failed', quality_action_skipped: 'Quality action skipped',
        ok: 'OK',
        network_fluctuation: 'Network Fluctuation',
        rate_limited: 'Rate Limited',
        server_error: 'Upstream Server Error',
        auth: 'Auth Failed',
        model_not_found: 'Model Not Found',
        invalid_response: 'Invalid Response',
        unsupported: 'Unsupported',
        manual_disable: 'Manually Disabled',
        manual_restore: 'Manually Restored',
        policy_unmanaged_restore: 'Policy unbound; upstream target restored to its original state',
        credential_unavailable: 'Upstream credentials cannot be obtained securely; probing is unavailable',
        secure_verification_required: 'Upstream Root verification is required before the channel key can be read',
        base_url_unavailable: 'No usable Base URL is available for probing',
        model_unavailable: 'No probe model is available; configure a model first',
        export_unavailable: 'The upstream account export endpoint is unavailable, so probe credentials cannot be obtained',
        credentials_redacted: 'Upstream credentials are redacted and cannot be used for probing'
      },
      topActions: {
        runFlow: 'How it works',
        policies: 'Automation Policies',
        events: 'Probe Events'
      },
      events: {
        title: 'Recent Probes & Remote Actions',
        empty: 'No events yet.',
        emptyForConnection: 'No events for this target yet.',
        showAll: 'Show All'
      },
      eventsDialog: {
        subtitle: 'Review probe health for each model of this probe target (account/channel).',
        globalSubtitle: 'Recent probe and remote action events.',
        viewingConnection: 'Viewing events for this target',
        card: {
          latencyLabel: 'Chat Latency',
          failureDurationLabel: 'Time to Failure',
          pingLabel: 'Node PING',
          availabilityLabel: 'Availability',
          recentRecordsLabel: 'Last 60 Records',
          past: 'PAST',
          now: 'NOW',
          noData: '-',
          nextProbeIn: 'Next probe: in {seconds}s',
          nextProbeDue: 'Next probe: due, waiting for scheduler',
          nextProbeNoPolicy: 'Next probe: no policy configured',
          nextProbeNeverProbed: 'Next probe: not probed yet',
          nextProbeDisabled: 'Next probe: disabled, not auto-probed',
          remoteActionLine: 'Remote action: {label}'
        }
      },
      remoteActions: {
        unsupported: 'Unsupported (no upstream call made)',
        skippedIndependentProbe: 'Skipped — auto remote action is not enabled',
        skippedTargetConflict: 'An upstream manual change was detected; automatic overwrite stopped',
        skippedTargetInitiallyDisabled: 'The target was already paused upstream and was not automatically enabled',
        sub2apiInactive: 'Sub2API account switched to inactive',
        sub2apiActive: 'Sub2API account switched to active',
        sub2apiInactiveFailed: 'Sub2API account switch to inactive failed',
        sub2apiActiveFailed: 'Sub2API account switch to active failed',
        newapiDisabled: 'NewAPI channel disabled',
        newapiUpdateFailed: 'Failed to update the NewAPI channel weight or status',
        newapiWeight: 'NewAPI channel weight set to {weight}',
        other: '{action}'
      },
      policies: {
        title: 'Automation Policies',
        subtitle: 'Configure model probing, auto-degradation, or multiplier-only priority behavior.',
        create: 'New Policy',
        empty: 'No automation policies yet. Click "New Policy" to configure one.',
        enabled: 'Enabled',
        disabled: 'Disabled',
        enable: 'Enable',
        disable: 'Disable',
        edit: 'Edit',
        delete: 'Delete policy',
        deleteTitle: 'Delete Automation Policy',
        deleteDescription: 'Delete "{name}"?',
        deleteWarning: 'Its model targets and account/group assignments will also be removed. Historical probe records will be retained. This action cannot be undone.',
        cancelDelete: 'Cancel',
        confirmDelete: 'Delete Policy',
        remoteActionOn: 'Remote Action On',
        allGroupsScope: 'All groups',
        modelTargetCount: '{count} model targets',
        strategyModes: {
          health_probe: 'Health Probe',
          multiplier_only: 'Multiplier Only'
        },
        multiplierOnlySummary: 'No probes; priority follows multiplier'
      },
      policyDrawer: {
        probeModeLabel: 'Probe method',
        sharedProbeModeHint: 'A shared channel/model is probed once using the effective policy. Groups share its result; history shows the actual method.',
        probeModes: { real_model: 'Lightweight request (existing)', arithmetic: 'Arithmetic check (Sub2API monitor style)', sub2api_test: 'Sub2API account test API', first_token: 'Streaming first token' },
        probeModeHints: {
          real_model: 'Directly calls this channel’s Chat Completions with hi by default and measures the complete response.',
          arithmetic: 'Sends a random addition/subtraction challenge directly to the channel. A correct answer is required; output is limited to 50 tokens. OpenAI uses Responses with a Chat Completions fallback for unsupported endpoints; Claude/Gemini use native protocols. This policy controls timeout and actions.',
          sub2api_test: 'Calls the connected Sub2API account test endpoint for this channel and model, sends hi and waits for completion. Uses Sub2API credentials, proxy and model mapping. Duration includes the network hop to Sub2API.',
          first_token: 'Sends hi by default and stops at the first nonempty answer text. Headers, heartbeats and reasoning do not count. Priorities use first-token latency; completion of the rest of the answer is not tested.',
        },
        nativeProbeNotice: 'Sub2API only. This endpoint may clear upstream errors, rate limits and temporary suspension on success or change account state on failure. Sub2API performs these actions independently of this policy’s suspension permission and consecutive thresholds.',
        maxFirstTokenLatencyLabel: 'Maximum time to first token (ms)',
        maxFirstTokenLatencyHelp: 'No answer text within this deadline counts as a failed probe. The request ends as soon as the first text arrives.',

        maxLatencyLabel: 'Maximum response latency (ms)',
        maxLatencyHelp: 'Maximum wait for a channel probe response. Defaults to 20000 ms (20 seconds); requests exceeding it are recorded as timeouts.',
        saving: 'Saving…',
        channels: {
          title: 'Participating channels',
          selected: '{selected}/{total} selected',
          hint: 'This selection applies to all policies and quality checks in this group only.',
          loading: 'Loading channel selection…',
          retry: 'Retry',
          selectAll: 'Select all',
          clear: 'Clear',
          search: 'Search channels',
          empty: 'No matching channels',
        },
        createTitle: 'New Automation Policy',
        editTitle: 'Edit Automation Policy',
        nameLabel: 'Policy Name',
        namePlaceholder: 'Enter policy name',
        enabledLabel: 'Enable this policy',
        strategyModeLabel: 'Run Mode',
        strategyModes: {
          health_probe: {
            title: 'Health Probe',
            description: 'Probes, state machine, and optional actions'
          },
          multiplier_only: {
            title: 'Multiplier Only',
            description: 'Adjust priority only; never probe'
          }
        },
        ownGroupLabel: 'Policy Scope',
        ownGroupAllOption: 'All groups in current workspace',
        modelTargetsLabel: 'Model Probe Targets',
        addModelTarget: 'Add Model',
        modelNamePlaceholder: 'Model name, e.g. gpt-4o-mini',
        modelEnabledLabel: 'Enabled',
        maxProbeTokensLabel: 'Max tokens',
        probePromptPlaceholder: 'Probe prompt (leave empty for default)',
        probeIntervalLabel: 'Probe Interval (seconds)',
        dailyBudgetLabel: 'Daily Probes per Channel',
        failureThresholdLabel: 'Failure Threshold',
        successThresholdLabel: 'Recovery Success Threshold',
        autoDegradeLabel: 'Auto Degrade',
        autoDegradeHelp: 'Update health from probes for priority sorting. Suspension requires separate permission.',
        autoRemoteActionLabel: 'Auto Remote Action',
        autoRemoteActionHelp: 'Upstream status/weight changes require both Auto Degrade and Allow Channel Suspension. Priority sorting works independently.',
        autoSuspendLabel: 'Allow Channel Suspension',
        autoSuspendHelp: 'Off by default. When off, probes only affect priority and never suspend channels. Enable to allow automatic suspension.',
        priorityModeLabel: 'Upstream Traffic Priority',
        priorityModes: {
          latency: 'By chat latency',
          none: 'Keep Upstream Values',
          multiplier: 'Sort by Group Multiplier'
        },
        priorityModeHelp: 'Latency sorting maps recent successful probes to configurable priorities. Multiplier sorting retains the existing rules. Failure and recovery thresholds still apply.',
        multiplierOnlySummaryTitle: 'Lower Multiplier, Higher Priority',
        multiplierOnlySummary: 'About every 30 seconds, the scheduler reads current group multipliers and syncs upstream priority. It never resolves probe credentials, requests models, consumes probe budget, degrades health, or runs remote health actions. Manual priority edits stop automatic overwrites.',
        providerLabel: 'Model Provider',
        providerPlaceholder: 'Select a provider',
        providerMismatchWarning: 'This policy\'s existing model targets use different providers. Pick one provider above — saving will unify all model targets to the provider you select.',
        cancel: 'Cancel',
        save: 'Save Policy',
        tooltips: {
          ownGroup: 'Describes the business group scope for this policy. Independent group-health probing enables policies through explicit target assignments and uses this policy\'s enabled model targets as the model pool. If the target has its own model list, the intersection of target models and the policy model pool is used; otherwise the policy model pool is used directly.',
          modelTargets: 'The models this policy probes. Both automatic scheduling and manual probes run against exactly these models.',
          provider: 'A probe policy can only use one provider (openai / anthropic / gemini / custom). Every model target added below automatically uses this provider, so a single policy never mixes providers.',
          probeInterval: 'The next probe is due one configured interval after the last probe started, whether it succeeded, failed, or the channel is suspended.',
          dailyBudget: 'Each channel has an independent quota under this policy. Automatic and policy-managed manual probes share it; each model request counts once. Shared channels are deduplicated across groups using the effective policy. Resets at 00:00 UTC+8. One model every 60 seconds uses about 1,440 probes per day.',
          failureThreshold: 'All probe failures count, including timeouts, network errors, rate limits and authentication failures. Health, priority or suspension changes at this consecutive failure threshold, according to the automation switches. A success resets the failure count.',
          successThreshold: 'Reaching this consecutive success threshold restores healthy status and the channel according to the automation switches. A failure resets the success count.',
          autoDegrade: 'Update health at the consecutive failure and success thresholds. With suspension disabled, health only affects priority. Disabling Auto Degrade only records results.',
          autoRemoteAction: 'Auto Degrade, Auto Remote Action and Allow Channel Suspension must all be enabled to change Sub2API account status or NewAPI channel status/weight. Upstream priority has its own setting.',
          autoSuspend: 'Allow local suspension at the consecutive failure threshold. Upstream suspension also requires Auto Degrade and Auto Remote Action. When off, only health and priority change; system-disabled channels return to their original state while manually disabled channels are preserved.',
          priorityMode: 'Latency mode uses recent successful probes, weights and configured bands. Multiplier mode retains the existing rules. Manual priority edits stop automatic overwrites.'
        },
        runFlow: {
          buttonLabel: 'How it works',
          title: 'Probe Health: How It Works',
          subtitle: 'A complete explanation for admins covering how policies, scheduling, the state machine, and manual probing relate to each other.',
          close: 'Close run flow explanation',
          steps: {
            policyScope: {
              title: '1. How a policy takes effect',
              description: 'Group health uses independent probing: probe targets are the accounts (Sub2API) / channels (NewAPI) inside admin groups of the current admin workspace, not real_connections links. An account/channel is auto-probed only after a policy is explicitly assigned to it. The probed models come from the enabled model targets on assigned policies; if the target carries its own model list, the intersection of target models and the policy model pool is used, otherwise the policy model pool is used directly.'
            },
            modelProvider: {
              title: '2. How model targets take effect',
              description: 'Each probe policy corresponds to exactly one provider (openai / anthropic / gemini / custom); every model target added under that policy belongs to that same provider, so a single policy never mixes models from multiple providers. Both automatic scheduling and manual probes fire probe requests against the candidate models derived in the previous step, one by one.'
            },
            schedulerCadence: {
              title: '3. Automatic scheduling cadence',
              description: 'A dedicated backend scheduler checks loaded channels for due probes every second, while upstream group and channel lists refresh independently in the background. The scheduling unit is "one probe target (account/channel) + one model" — multiple candidate models under the same target are split into separate tasks that are each evaluated independently.'
            },
            dueCheck: {
              title: '4. How "due" is determined',
              description: 'A target/model that has never been probed is scheduled as soon as possible. Subsequent probes are due one configured interval after the last probe started. Successful, failed and suspended targets use the same interval.'
            },
            budget: {
              title: '5. Budget rules',
              description: 'The policy limit applies independently to each channel. Automatic and policy-managed manual probes share that quota; each model request counts once. Shared channels are deduplicated across groups using the effective policy. At the limit, only that channel under that policy stops probing, with usage and reset time shown. Resets at 00:00 UTC+8. One model every 60 seconds uses about 1,440 probes per day.'
            },
            stateTransition: {
              title: '6. State transitions',
              description: 'With Auto Degrade enabled, all failures count toward the consecutive failure threshold before changing health and applying the configured actions. With suspension disabled, health only affects priority. With suspension enabled, the threshold can suspend the channel. Success resets consecutive failures; failure resets consecutive successes.'
            },
            recoveryThreshold: {
              title: '7. Recovery at the success threshold',
              description: 'Suspended channels continue probing at the configured interval. Reaching the consecutive success threshold restores healthy status and, when automation permits, restores the original channel status and weight. Every controlled model must be healthy before a channel with multiple models is restored.'
            },
            autoDegradeVsRemoteAction: {
              title: '8. Auto Degrade vs. Auto Remote Action',
              description: 'Auto Degrade updates health from probe results. Allow Channel Suspension is off by default: health only affects enabled priority sorting, without disabling channels or reducing forwarding weight. Auto Degrade, Auto Remote Action and Allow Channel Suspension must all be enabled to change Sub2API account status or NewAPI channel status/weight. Priority sorting has its own setting.'
            },
            manualProbe: {
              title: '9. Manual probing',
              description: 'Manual probing uses the effective policy models, method, latency limit and daily budget. Results count toward failures, recovery and weighted latency, and trigger permitted actions. Unmanaged channels can be tested with history only. Manual quality tests use global settings and honor health suspension. Group probes on the left use a separate group key through Sub2API and save their own results independently.'
            },
            nextProbeCopy: {
              title: '10. What "next probe" copy means',
              description: 'Due, waiting for scheduler means the configured interval has elapsed. Execution still requires a scheduler scan, available concurrency and remaining daily probe budget.'
            }
          }
        },
        errors: {
          nameRequired: 'Please enter a policy name.',
          modelTargetRequired: 'At least one probe target with a model name is required.',
          providerRequired: 'Please select a provider for this policy.'
        }
      },
      probeDialog: {
        title: 'Select Probe Models',
        cancel: 'Cancel',
        confirm: 'Start Probe',
        emptyTitle: 'This target has no probeable models.',
        emptyHint: 'Add and enable model targets in a probe policy first.',
        fromPolicy: 'From policy "{name}"',
        maxTokens: 'Max {count} tokens',
        remoteActionOn: 'Remote Action On',
        noResults: 'Probe ran but returned no results. Try again later or check upstream site connectivity.'
      },
      manualProbeDialog: {
        title: 'Manual One-Time Probe',
        loadingModels: 'Fetching available models from upstream...',
        retryLoad: 'Retry',
        empty: 'No available models were found.',
        selectHint: 'Select models to test with the effective policy. Results count toward failures, recovery and latency, and appear in history. Without an active policy, tests only record history.',
        startTest: 'Start Test',
        testing: 'Testing...',
        resultTitle: 'Test Results',
        resultEmpty: 'No test run yet. Select models and click "Start Test".',
        latency: '{ms}ms',
        selectedCount: '{count} model(s) selected',
        close: 'Close'
      },
      policyAssignment: {
        title: 'Assign Probe Policies',
        subtitle: 'manage automatic probing assignments',
        save: 'Save',
        cancel: 'Cancel',
        empty: 'No probe policies in this workspace yet. Create one first.'
      },
      errors: {
        request: 'Operation failed. Please try again.',
        unknown: 'Group health data is temporarily unavailable. Please try again.',
        network: 'Network error. Check your connection and try again.',
        notFound: 'Probe target not found or inaccessible.',
        noMatchingModels: 'Selected models do not match the current probe policy. Reopen the probe dialog.',
        probeBudgetExhausted: 'This channel has reached its daily probe limit under the effective policy. Resets at 00:00 UTC+8. No request was sent.',
        accountsFetch: 'Failed to load accounts for this group.',
        targetNotFound: 'Probe target not found or not in the current workspace.',
        credentialUnavailable: 'Cannot securely obtain upstream credentials; probing unavailable.',
        secureVerificationRequired: 'Upstream root secure verification is required to read the channel key.',
        baseUrlUnavailable: 'No usable Base URL; probing unavailable.',
        modelUnavailable: 'No probe model available; configure one in a probe policy first.',
        exportUnavailable: 'Upstream account export endpoint unavailable; cannot obtain credentials.',
        credentialsRedacted: 'Upstream credentials are redacted and cannot be used for probing.',
        modelListUnavailable: 'Could not fetch the upstream model list. Please try again later.',
        modelListInvalid: 'The upstream model list response format is not recognized.',
        latencyPriorityInvalid: 'Invalid latency configuration: check samples, weights, model, priorities, and contiguous ranges from 0 to an unbounded final band.',
        probeModeInvalid: 'Select a valid probe method.',
        maxLatencyInvalid: 'Enter a valid positive integer for maximum response latency (milliseconds).',
        multiplierRequired: 'This group has no valid multiplier. Set it upstream before enabling multiplier sorting.',
        manualModelsRequired: 'Please select at least one model before starting the test.',
        policyNotFound: 'The selected policy does not exist or is not in the current workspace.'
      }
    },
      upstream: {
        searchPlaceholder: 'Search site name...',
        addSite: 'Add Site',
        summary: '{connected} / {total} upstream sites connected',
        refresh: {
          action: 'Refresh Data',
          refreshing: 'Refreshing...',
          countdown: 'Refresh in {seconds}s',
          disabled: 'Auto refresh disabled'
        },
      modal: {
        title: 'Add Upstream Site',
        editTitle: 'Edit Upstream Site',
        cancel: 'Cancel',
        submit: 'Add Site',
        updateSubmit: 'Save Changes',
        submitting: 'Connecting...',
        form: {
          siteName: 'Site Name',
          siteNamePlaceholder: 'Enter site name',
          siteUrl: 'Site URL',
          siteUrlPlaceholder: 'Enter full site URL, e.g. https://api.example.com',
          platform: 'Platform',
          platforms: {
            auto: 'Auto Detect',
            sub2api: 'Sub2API',
            newapi: 'New-API'
          },
          authMode: 'Authentication Method',
          authModes: {
            password: 'Account Password',
            passwordHelp: 'Sign in with the site account and password, then save the session automatically.',
            token: 'Access Token / Refresh Token',
            tokenHelp: 'Use this for Sub2API sites where Cloudflare or two-factor login blocks direct password login.',
            userKey: 'User Key',
            userKeyHelp: 'Use the system access token generated in New-API personal settings.'
          },
          account: 'Account',
          accountPlaceholder: 'Enter account',
          password: 'Password',
          passwordPlaceholder: 'Enter password',
          passwordEditPlaceholder: 'Leave blank to keep current password',
          passwordEditHelp: 'Leaving this blank keeps the current login session. Enter a new password only when you want to re-login and update credentials.',
          accessToken: 'Access Token',
          accessTokenPlaceholder: 'Paste access_token, or leave blank and provide only refresh_token',
          refreshToken: 'Refresh Token',
          refreshTokenPlaceholder: 'Paste refresh_token; the server refreshes it once for the latest expiry',
          tokenType: 'Token Type',
          tokenTypePlaceholder: 'Defaults to Bearer',
          tokenHelp: 'When refresh_token is provided, the server refreshes it first to obtain the latest access_token and expiration time.',
          userId: 'User ID',
          userIdPlaceholder: 'Enter the user ID that owns the system access token',
          userKey: 'System Access Token',
          userKeyPlaceholder: 'Paste the New-API system access token',
          userKeyHelp: 'Balance, groups, and usage are requested with Authorization Bearer and New-Api-User.',
          rechargeRate: 'Recharge Multiplier',
          rechargeRatePlaceholder: 'Enter the USD to CNY multiplier, e.g. 7',
          rechargeRateHelp: 'Required. CNY amount = USD amount × this multiplier.',
          remark: 'Remark',
          remarkPlaceholder: 'Enter remark (optional)'
        }
      },
      currency: {
        usdValue: '{amount} USD',
        cnyValue: '{amount} CNY'
      },
      fields: {
        siteName: 'Site Name',
        siteUrl: 'Site URL',
        platform: 'Platform',
        balance: 'Balance',
        todayConsume: 'Today Consume',
        historyRecharge: 'History Recharge',
        groupName: 'Group Name',
        groupMultiplier: 'Group Multiplier',
        availableGroups: 'Available Groups',
        viewAvailableGroups: 'View Available Groups',
        closeGroupsModal: 'Close',
        dedicatedMultiplierBadge: 'Dedicated Rate',
        dedicatedMultiplierTooltip: 'This user has a sub2api dedicated rate override. Business calculations use the rate on the right.',
        unknownPlatform: 'Unknown Platform',
        isConnected: 'Integration',
        connected: 'Connected',
        disconnected: 'Disconnected',
        lastUpdated: 'Last Updated',
        notSynced: 'Not synced yet'
      },
      status: {
        connecting: 'Connecting',
        syncing: 'Syncing',
        connected: 'Connected',
        error: 'Error'
      },
      empty: {
        title: 'No upstream sites found',
        description: 'Adjust your search or add an upstream site.'
      },
      delete: {
        action: 'Delete site',
        title: 'Delete this upstream site?',
        description: 'You are deleting "{name}". To restore it, you will need to add and log in again.',
        cancel: 'Cancel',
        confirm: 'Delete'
      },
      action: {
        sync: 'Sync',
        syncing: 'Syncing',
        edit: 'Edit Site',
        settings: 'Site Settings',
        actions: 'Actions'
      },
      siteSettings: {
        title: 'Site Alert Settings',
        balanceThreshold: 'Custom Balance Threshold',
        balanceThresholdHelp: 'Enable to use a site-specific threshold. Disable to use the global default.',
        balanceThresholdPlaceholder: 'Enter threshold amount',
        save: 'Save',
        saveSuccess: 'Saved',
        saving: 'Saving...',
        cancel: 'Cancel'
      },
      viewMode: {
        list: 'List Mode',
        card: 'Card Mode'
      },
      syncStream: {
        syncing: 'Syncing...',
        done: 'Sync complete',
        error: 'Sync failed',
      },
      errors: {
        invalidUrl: 'The site URL is invalid. Check it and try again.',
        network: 'Network or CORS request failed. Check the site URL and cross-origin settings.',
        auth: 'Login failed. Check the account or password.',
        request: 'The upstream API request failed. Try again later.',
        invalidResponse: 'The upstream response could not be parsed.',
        tokenMissing: 'Login succeeded but no access token was returned.',
        detect: 'The platform could not be auto-detected. Choose a platform and try again.',
        sub2APIBulkUpdateUnsupported: 'This Sub2API version does not support safe account field updates. Upgrade Sub2API and try again.',
        unknown: 'An unknown error occurred while connecting the upstream site.'
      }
    },
    groupRates: {
      badge: 'Rate Sync Ledger',
      title: 'Group Rates',
      subtitle: 'Review current upstream group multipliers and recent changes, then inspect multiplier history.',
      common: {
        placeholder: '—',
        allTypes: 'All Types',
        allPlatforms: 'All Platforms',
        unknown: 'Unknown'
      },
      platforms: {
        newapi: 'New-API',
        sub2api: 'Sub2API'
      },
      summary: {
        totalLabel: 'Total Group Rates',
        updatedLabel: 'Synced Records'
      },
      table: {
        title: 'Rate List',
        description: 'Rows preserve the ordering returned by the backend.'
      },
      fields: {
        siteName: 'Site Name',
        groupName: 'Group Name',
        type: 'Group Type',
        platform: 'Site Platform',
        currentMultiplier: 'Current Multiplier',
        effectiveMultiplier: 'Converted Cost Multiplier',
        multiplierFormula: 'Upstream {upstream} × recharge factor {recharge}',
        delta: 'Rise/Fall',
        updatedAt: 'Updated Time',
        actions: 'Actions'
      },
      actions: {
        refresh: 'Refresh Data',
        createCampaign: 'Create Campaign',
        viewHistory: 'View History',
        viewHistoryForRate: 'View rise/fall history for {site} · {group}; current change {delta}',
        closeHistory: 'Close History',
        editType: 'Edit',
        closeEdit: 'Close Group Type Editor',
        connect: 'Configure Connection',
        closeConnect: 'Close connect dialog',
        saveConnect: 'Connect',
        cancel: 'Cancel',
        saveType: 'Save Type'
      },
      filters: {
        searchLabel: 'Search',
        searchPlaceholder: 'Search site or group...',
        typeLabel: 'Group Type',
        platformLabel: 'Site Platform'
      },
      sort: {
        label: 'Sort',
        multiplierAsc: 'Multiplier Low to High',
        multiplierDesc: 'Multiplier High to Low',
        siteNameAsc: 'Site Name A-Z',
        groupNameAsc: 'Group Name A-Z'
      },
      tabs: {
        all: 'All',
        mapped: 'Mapped',
        unmapped: 'Unmapped',
        deleted: 'Deleted'
      },
      pagination: {
        previous: 'Previous',
        next: 'Next',
        currentPage: 'Page {page} / {totalPages}',
        total: '{total} total',
        pageSize: '{pageSize} per page'
      },
      status: {
        loading: 'Loading group rates...',
        mapped: 'Connected',
        pricingMapped: 'Pricing Source',
        unmapped: 'Not Connected',
        deleted: 'Deleted'
      },
      empty: {
        title: 'No group rates yet',
        description: 'Synced upstream group multiplier data will appear here.',
        filteredTitle: 'No records match these filters',
        filteredDescription: 'Change the status, search, type, or platform filter.'
      },
      history: {
        title: 'Multiplier History',
        titleWithGroup: '{site} · {group} Multiplier History',
        subtitle: 'Platform: {platform}',
        loading: 'Loading history records...',
        emptyTitle: 'No history records',
        emptyDescription: 'This site group has not returned multiplier history yet.',
        multiplier: 'Multiplier',
        delta: 'Rise/Fall',
        createdAt: 'Recorded Time'
      },
      edit: {
        title: 'Edit Group Type',
        titleWithGroup: 'Edit Group Type for {site} · {group}',
        description: 'Saving updates the multiplier type for this site group and refreshes the list.',
        typeLabel: 'Group Type',
        typePlaceholder: 'Select group type'
      },
      connect: {
        titleWithGroup: 'Configure {site} · {group}',
        description: 'Let the system create both resources, or link resources that already exist.',
        ownGroupLabel: 'My Site Groups',
        ownGroupPlaceholder: 'Select my site groups',
        upstreamGroupLabel: 'Connected Group',
        upstreamGroupPlaceholder: 'Select connected group',
        upstreamSiteLabel: 'Upstream Site',
        upstreamGroupNameLabel: 'Upstream Group',
        upstreamMultiplierLabel: 'Upstream Multiplier',
        upstreamPlatformLabel: 'Platform',
        modeData: 'Data Stats',
        modeReal: 'Create Resources',
        realDescription: 'Create an upstream Key/Token and an account or channel on the current admin site.',
        groupTypeLabel: 'Group Type',
        groupTypePlaceholder: 'Select group type',
        groupTypeOpenai: 'OpenAI',
        groupTypeAnthropic: 'Anthropic',
        groupTypeGemini: 'Gemini',
        groupTypeAntigravity: 'Antigravity',
        channelTypeLabel: 'Channel Type',
        channelTypePlaceholder: 'Select channel type',
        realNotSupported: 'Real connect is not supported for this platform',
        realConnecting: 'Creating connection...',
        realSuccess: 'Real connection created successfully',
        realFailed: 'Failed to create real connection',
        modeBind: 'Use Existing Resources',
        bindDescription: 'Link an existing upstream Key/Token and admin account/channel without taking ownership of them.',
        bindSelectKey: 'Upstream Key / Token',
        bindKeysLoading: 'Loading credentials for this group...',
        bindKeysEmpty: 'No credentials are available for this upstream group',
        bindSelectAdminGroup: 'Admin Group',
        bindAdminGroupPlaceholder: 'Select the group containing the existing resource',
        bindSelectAdminResource: 'Existing Account / Channel',
        adminResourcesLoading: 'Loading admin resources...',
        adminResourcesEmpty: 'No accounts or channels are available in this group',
        adminResourcesFailed: 'Failed to load existing admin resources. Refresh and try again.',
        resourceActive: 'Active',
        resourceInactive: 'Inactive',
        addToPricingMapping: 'Also use as a pricing source',
        addToPricingMappingHint: 'Enabled by default. Turn it off to create only the traffic connection.',
        submitManaged: 'Create and Connect',
        submitExisting: 'Save Existing Link',
        bindFailed: 'Failed to link existing resources'
      },
      disconnect: {
        action: 'Disconnect',
        title: 'Disconnect',
        description: 'Confirm disconnecting {site} · {group}?',
        unlinkOnly: 'Unlink Only',
        unlinkOnlyHint: 'Only remove the local binding record, keep the upstream Key and Admin account',
        deleteAll: 'Delete Account & Key',
        deleteAllHint: 'Also delete the upstream Key and the Admin site forwarding account',
        removePricingMapping: 'Also remove the pricing source',
        removePricingMappingHint: 'Turn this off to keep the upstream group in pricing mappings.',
        confirm: 'Confirm',
        disconnecting: 'Disconnecting...',
        failed: 'Failed to disconnect'
      },
      format: {
        multiplier: '{value}x',
        deltaMultiplier: '{value}x'
      },
      errors: {
        network: 'Network or CORS request failed. Check the API URL and cross-origin settings.',
        request: 'The group rates API request failed. Try again later.',
        unknown: 'An unknown error occurred while loading group rates.',
        refreshFailed: 'The change was saved, but the list refresh failed. Refresh again to update the view.'
      }
    },
    groupRateCampaigns: {
      title: 'Rate Campaigns',
      subtitle: 'Batch-adjust your own group multipliers, with scheduled start/end and automatic restore.',
      common: {
        placeholder: '—'
      },
      actions: {
        create: 'New Campaign',
        refresh: 'Refresh',
        start: 'Start Now',
        end: 'End Campaign',
        cancel: 'Cancel Campaign',
        viewDetail: 'View Details',
        close: 'Close',
        preview: 'Preview Impact',
        confirmCreate: 'Create Campaign',
        cancelEdit: 'Cancel'
      },
      tabs: {
        all: 'All'
      },
      status: {
        draft: 'Draft',
        scheduled: 'Scheduled',
        running: 'Running',
        ending: 'Ending',
        ended: 'Ended',
        partial: 'Partial',
        failed: 'Failed',
        cancelled: 'Cancelled',
        loading: 'Loading campaigns...'
      },
      fields: {
        name: 'Campaign Name',
        status: 'Status',
        startAt: 'Start Time',
        endAt: 'End Time',
        summary: 'Result',
        createdBy: 'Created By',
        actions: 'Actions'
      },
      empty: {
        title: 'No campaigns yet',
        description: 'Click "New Campaign" to create your first rate campaign.'
      },
      pagination: {
        total: '{total} total',
        pageSize: '{pageSize} per page',
        currentPage: 'Page {page} / {totalPages}',
        previous: 'Previous',
        next: 'Next'
      },
      format: {
        summary: '{applied}/{total} applied'
      },
      errors: {
        network: 'Network or CORS request failed. Check the API URL and cross-origin settings.',
        request: 'The rate campaigns API request failed. Try again later.',
        unknown: 'An unknown error occurred while loading rate campaigns.',
        emptySelection: 'Select at least one group, and every group must exist in your own groups.',
        invalidName: 'Invalid campaign name. It must be between 1 and 80 characters.',
        invalidAdjustment: 'Invalid campaign multiplier. Check that every selected group has a valid fixed rate.',
        invalidSchedule: 'Invalid schedule. Please check the start/end time settings.',
        noNotifyBots: 'Select at least one bot when notifications are enabled.',
        notFound: 'Campaign not found.',
        invalidState: 'This action is not allowed for the campaign\'s current status.',
        duplicateGroup: 'The same group cannot be selected more than once.'
      },
      editor: {
        titleCreate: 'New Rate Campaign',
        sectionInfo: 'Campaign Info',
        nameLabel: 'Campaign Name',
        namePlaceholder: 'e.g. Double 11 Sale',
        descriptionLabel: 'Description',
        descriptionPlaceholder: 'Optional, for your own reference',
        sectionSelection: 'Select Groups',
        selectionHint: 'Set a campaign multiplier for each group',
        groupsEmpty: 'No groups available',
        groupMultiplierPlaceholder: 'Campaign multiplier',
        sectionSchedule: 'Schedule',
        startModeLabel: 'Start Mode',
        startNow: 'Start Now',
        startScheduled: 'Scheduled Start',
        startDraft: 'Save as Draft',
        startAtLabel: 'Start Time',
        endModeLabel: 'End Mode',
        endScheduled: 'Scheduled End',
        endManual: 'Manual End',
        endAtLabel: 'End Time',
        sectionNotify: 'Notifications',
        notifyEnableLabel: 'Enable Bot Notifications',
        notifyBotSelectLabel: 'Select Bots',
        notifyNoBots: 'No bots available. Configure one in system settings first.',
        notifyStartTemplateLabel: 'Start Notification Text',
        notifyEndTemplateLabel: 'End Notification Text',
        notifyVariablesTitle: 'Available variables, click to copy',
        notifyVarActivityName: 'Campaign name',
        notifyVarTotalCount: 'Total target groups',
        notifyVarAppliedCount: 'Applied count',
        notifyVarFailedCount: 'Failed count',
        notifyVarStartTime: 'Start time',
        notifyVarEndTime: 'End time',
        copyVarFailed: 'Copy failed. Please copy the variable manually.',
        previewTitle: 'Preview Affected Groups',
        previewEmpty: 'No preview yet. Click "Preview Impact" to see affected groups.',
        previewGroupName: 'Group Name',
        previewOriginal: 'Original',
        previewCampaign: 'Campaign',
        previewTotal: '{total} groups affected',
        errors: {
          nameRequired: 'Please enter a campaign name',
          selectionEmpty: 'Please select at least one group',
          groupMultiplierInvalid: 'Enter a valid campaign multiplier for every group',
          scheduleInvalid: 'Please check the start/end time settings',
          notifyBotsRequired: 'Please select at least one bot when notifications are enabled'
        }
      },
      detail: {
        title: 'Campaign Details',
        sectionConfig: 'Campaign Config',
        sectionItems: 'Group Details',
        itemGroupName: 'Group Name',
        itemOriginal: 'Original',
        itemCampaign: 'Campaign',
        itemRestored: 'Restored',
        itemApplyStatus: 'Start Status',
        itemRestoreStatus: 'Restore Status',
        noReason: '—',
        confirmEnd: 'Are you sure you want to manually end this campaign? All groups will be restored to their original multipliers immediately.',
        confirmCancel: 'Are you sure you want to cancel this campaign? No pricing changes will be applied.'
      }
    },
    mySites: {
      errors: {
        invalidAutoPricingConfig: 'Invalid auto-pricing config: primary upstream not in linked upstreams, or min multiplier exceeds max.',
        connectionExists: 'A real connection already exists for this upstream group.',
        managedDeleteOnly: 'Existing-resource links can only be unlinked locally; remote resources cannot be deleted.'
      }
    },
    tickets: {
      title: 'Tickets',
      subtitle: 'View and reply to user tickets submitted through the embedded iframe.',
      common: {
        placeholder: '—'
      },
      actions: {
        refresh: 'Refresh',
        viewDetail: 'View Details',
        embedSettings: 'Embed Settings'
      },
      tabs: {
        all: 'All'
      },
      status: {
        open: 'Open',
        pending: 'Pending',
        replied: 'Replied',
        closed: 'Closed',
        loading: 'Loading tickets...'
      },
      fields: {
        title: 'Title',
        status: 'Status',
        category: 'Category',
        priority: 'Priority',
        manualEmail: 'Contact Email',
        sub2apiUser: 'Sub2API User',
        sub2apiSrcHost: 'Source Host',
        lastMessageAt: 'Last Reply',
        actions: 'Actions'
      },
      empty: {
        title: 'No tickets yet',
        description: 'This workspace has not received any tickets yet.'
      },
      pagination: {
        total: '{total} total',
        pageSize: '{pageSize} per page',
        currentPage: 'Page {page} of {totalPages}',
        previous: 'Previous',
        next: 'Next'
      },
      errors: {
        network: 'Network or CORS request failed. Check the API URL and cross-origin settings.',
        request: 'The ticket request failed. Try again later.',
        unknown: 'An unknown error occurred while loading tickets.',
        notFound: 'Ticket not found.',
        invalidStatus: 'Invalid ticket status.',
        bodyRequired: 'Reply content is required.',
        ticketClosed: 'This ticket is closed and can no longer be replied to.',
        noCurrentAccount: 'Please select a workspace first.',
        invalidTemplate: 'Unsupported embed page template.',
        invalidMaxImages: 'Max images per ticket must be between 0 and 9.',
        attachmentLoadFailed: 'Failed to load image, please try again later.',
        invalidCategoryOptions: 'Invalid category options. Check for empty, duplicate, or over-limit entries.',
        invalidPriorityOptions: 'Invalid priority options. Check for empty, duplicate, or over-limit entries.'
      },
      detail: {
        title: 'Ticket Details',
        sectionTicket: 'Ticket Info',
        sectionMessages: 'Messages',
        sectionReply: 'Reply',
        category: 'Category',
        priority: 'Priority',
        manualEmail: 'Contact Email',
        lastMessageAt: 'Last Reply',
        sub2apiUserId: 'Sub2API User ID',
        sub2apiEmail: 'Sub2API Email',
        sub2apiRole: 'Sub2API Role',
        sub2apiSrcHost: 'Source Host',
        authorAdmin: 'Support',
        authorCustomer: 'User',
        replyPlaceholder: 'Type your reply...',
        send: 'Send Reply',
        attachmentLoadFailed: 'Failed to load image',
        previewImage: 'Zoom preview',
        closePreview: 'Close preview'
      },
      embedConfig: {
        title: 'Embed Settings',
        sections: {
          basic: 'Basic Settings',
          category: 'Category',
          priority: 'Priority'
        },
        legacyNotice: 'The "Enable Embedded Tickets" and "Allowed Source Host" settings have been removed. The embed URL is always active; contact an administrator if you need to restrict access.',
        embedUrl: 'Embed URL',
        embedUrlHint: 'Configure this URL in the Sub2API custom iframe. Sub2API automatically appends user identity parameters.',
        copy: 'Copy',
        copied: 'Copied',
        copyFailed: 'Copy failed, please copy manually.',
        openPreview: 'Open Ticket Page',
        openPreviewHint: 'Opens the embed page in a new tab for preview. Identity parameters will be missing outside a real Sub2API iframe — that is expected.',
        template: 'Page Template',
        templates: {
          default: {
            name: 'Default Compact',
            description: 'Standard rounded card style, suitable for default use.'
          },
          minimal: {
            name: 'Minimal',
            description: 'A lighter visual density, suitable for embedding into an existing admin style.'
          },
          support: {
            name: 'Support Panel',
            description: 'A more conversational look, suitable for use as a standalone support panel.'
          }
        },
        maxImages: 'Max Images Per Ticket',
        maxImagesHint: '0 disables image uploads. Maximum 9 images.',
        categoryOptions: 'Category Options',
        priorityOptions: 'Priority Options',
        addOption: 'Add Option',
        addOptionPlaceholder: 'Type a new option and click add',
        removeOption: 'Remove this option',
        restoreDefaults: 'Restore Defaults',
        optionsHint: 'Keep at least 1 option, up to 40 characters each. Customers must pick from these when creating a ticket.',
        saveTemplate: 'Save Settings',
        saving: 'Saving...',
        saveSuccess: 'Settings saved',
        rotateToken: 'Rotate Embed URL',
        confirmRotate: 'Are you sure you want to rotate the embed URL? The old embed URL will stop working immediately.'
      },
      sub2apiProfile: {
        title: 'Sub2API User Profile',
        sectionIdentity: 'Identity',
        userId: 'User ID',
        email: 'Email',
        role: 'Role',
        srcHost: 'Source Host',
        username: 'Username',
        status: 'Account Status',
        sectionBalance: 'Balance & Recharge',
        balance: 'Current Balance',
        totalRecharged: 'Total Recharged',
        registeredAt: 'Registered At',
        frozenBalance: 'Frozen Balance',
        concurrency: 'Concurrency',
        rpmLimit: 'RPM Limit',
        lastUsedAt: 'Last Used At',
        unavailable: 'Not available',
        sectionRechargeHistory: 'Recharge History',
        rechargeHistoryComingSoon: 'Not provided yet',
        historyEmpty: 'No recharge history yet',
        empty: 'No data',
        remoteUnavailable: {
          noUserId: 'This ticket has no recorded Sub2API user ID, so live data cannot be queried.',
          noAdminSession: 'The current workspace is not logged into a Sub2API admin account. Showing ticket snapshot data only.',
          userNotFound: 'Could not fetch live data for this user from Sub2API. Showing ticket snapshot data only.'
        }
      }
    },
    massEmail: {
      common: {
        placeholder: '-'
      },
      filters: {
        search: 'Search users',
        searchPlaceholder: 'Enter an email keyword',
        noSearch: 'No search term',
        status: 'User Status',
        role: 'Role',
        allStatuses: 'All statuses',
        allRoles: 'All roles'
      },
      template: {
        label: 'Email Template',
        placeholder: 'Select a template',
        noSubject: 'No subject selected'
      },
      selection: {
        title: 'Recipient Selection',
        count: '{count} selected across pages',
        selectPage: 'Select all users on this page',
        selectUser: 'Select {email}'
      },
      users: {
        title: 'Recipients'
      },
      fields: {
        email: 'Email',
        role: 'Role',
        status: 'Status',
        createdAt: 'Created',
        actions: 'Actions'
      },
      roles: {
        user: 'User',
        admin: 'Admin'
      },
      userStatus: {
        active: 'Active',
        disabled: 'Disabled',
        inactive: 'Inactive',
        banned: 'Banned'
      },
      actions: {
        search: 'Search',
        clearSearch: 'Clear search',
        refresh: 'Refresh',
        clearSelection: 'Clear',
        sendSelected: 'Send selected',
        sendPage: 'Send page',
        sendFilter: 'Send filter',
        sendRow: 'Send',
        cancelBatch: 'Cancel',
        closeConfirm: 'Close confirmation',
        openBatches: 'Batches',
        openBatchDetail: 'Details',
        previewTemplate: 'Preview'
      },
      status: {
        loadingUsers: 'Loading recipients...',
        loadingItems: 'Loading outcomes...'
      },
      batchStatus: {
        queued: 'Queued',
        running: 'Running',
        completed: 'Completed',
        completed_with_errors: 'Completed with errors',
        failed: 'Failed',
        cancelled: 'Cancelled',
        cancelling: 'Cancelling'
      },
      itemStatus: {
        pending: 'Pending',
        sending: 'Sending',
        sent: 'Sent',
        failed: 'Failed',
        uncertain: 'Uncertain',
        cancelled: 'Cancelled'
      },
      empty: {
        usersTitle: 'No recipients match this filter',
        usersDescription: 'Change the search, status, or role filter and try again.',
        batches: 'No mass-email batches yet.',
        detail: 'Select a batch to view recipient outcomes.'
      },
      pagination: {
        total: '{total} total',
        pageSize: '{pageSize} per page',
        currentPage: 'Page {page} of {totalPages}',
        previous: 'Previous',
        next: 'Next'
      },
      batches: {
        title: 'Batches',
        active: '{count} active',
        progress: '{done}/{total} done, {percent}%',
        close: 'Close batches'
      },
      detail: {
        title: 'Batch Detail',
        recipients: '{total} recipient outcomes',
        close: 'Close batch detail'
      },
      preview: {
        title: 'Template Preview',
        close: 'Close preview',
        iframeTitle: 'Email template preview'
      },
      summary: {
        sent: 'Sent',
        failed: 'Failed',
        uncertain: 'Uncertain',
        cancelled: 'Cancelled'
      },
      confirm: {
        selectedTitle: 'Send email to selected recipients?',
        selectedDescription: 'This will create a mass-email batch for {count} selected recipients.',
        allTitle: 'Send email to all recipients in the current filter?',
        allDescription: 'This will create a mass-email batch for {count} recipients matching the current filter.',
        recipients: 'Recipients: {count}',
        template: 'Template: {name}',
        filters: 'Filters: {status}, {role}, search: {search}',
        cancel: 'Cancel',
        submit: 'Create batch'
      },
      success: {
        created: 'Mass-email batch created.',
        cancelled: 'Batch cancellation requested.'
      },
      errors: {
        network: 'Network or CORS request failed. Check the API URL and cross-origin settings.',
        request: 'Mass-email request failed. Try again later.',
        templates: 'Failed to load email templates.',
        unknown: 'An unknown error occurred while loading mass-email data.',
        invalidRequest: 'The mass-email request is invalid.',
        invalidSelection: 'Select at least one valid recipient.',
        templateNotFound: 'The selected email template no longer exists.',
        smtpNotReady: 'SMTP settings are not ready.',
        upstreamAuth: 'Upstream admin authentication failed.',
        upstreamRequest: 'Upstream request failed.',
        notFound: 'Mass-email batch not found.',
        invalidState: 'This batch cannot perform the requested action in its current state.',
        persistence: 'Mass-email data could not be saved.',
        sendFailed: 'Email sending failed.',
        activeBatchExists: 'Another mass-email batch is already active in this workspace. Cancel or wait for it to finish before creating a new one.',
        recipientLimitReached: 'This selection exceeds the 10,000-recipient limit. Narrow the filters and try again.',
        itemGeneric: 'Recipient delivery failed. Check the batch later for updated details.'
      }
    },
    settings: {
      title: 'System Settings',
      subtitle: 'Manage system parameters, notification channels, and automation strategies.',
      save: 'Save Settings',
      saving: 'Saving...',
      saveSuccess: 'Saved',
      strategyDescription: 'Configure data refresh interval, alert thresholds, and automation strategies.',
      requiresRefresh: 'Enable data refresh interval first so the system can detect changes automatically.',
      balanceWarningAmount: 'Trigger Amount (CNY)',
      notifyBots: 'Send Notifications To',
      customTemplate: 'Custom Template',
      templateEditor: {
        formatLabel: 'Template format',
        formatHelp: 'The live preview uses sample values. Delivery is adapted to each notification channel\'s supported rich-text format.',
        editor: 'Template content',
        preview: 'Live preview',
        previewTitle: 'Live notification template preview',
        formats: {
          text: 'Plain text',
          markdown: 'Markdown',
          html: 'HTML'
        },
        samples: {
          siteName: 'Example upstream',
          balance: '8.50',
          threshold: '10.00',
          groupName: 'Default group',
          oldRate: '1.0000',
          newRate: '1.1200',
          changeDirection: 'increased'
        }
      },
      balanceTemplateVars: '(Variables: {siteName}, {balance}, {threshold})',
      multiplierTemplateVars: '(Variables: {siteName}, {groupName}, {oldRate}, {newRate}, {changeDirection})',
      unnamedBot: 'Unnamed Bot',
      noBotsConfigured: 'Configure bots in "Channels & Alerts" first',
      mustSelectBot: 'At least one notification bot must be selected',
      varSiteName: 'Site name',
      varBalance: 'Current balance (CNY)',
      varThreshold: 'Threshold (CNY)',
      varGroupName: 'Group name',
      varOldRate: 'Old rate',
      varNewRate: 'New rate',
      varChangeDirection: 'Change direction',
      pricingAmount: 'Adjustment Amount',
      botNameLabel: 'Bot Name',
      botNameDingtalkPlaceholder: 'e.g., DingTalk Main',
      botNameWecomPlaceholder: 'e.g., WeCom Main',
      botNameQQPlaceholder: 'e.g., QQ Direct Alerts',
      botNameFeishuPlaceholder: 'e.g., Feishu Main',
      botNameTelegramPlaceholder: 'e.g., TG Main',
      addDingtalkBot: 'Add DingTalk Bot',
      addWecomBot: 'Add WeCom Bot',
      addQQBot: 'Add QQ Bot',
      addFeishuBot: 'Add Feishu Bot',
      addTelegramBot: 'Add TG Bot',
      emptyDingtalk: 'No DingTalk bots configured',
      emptyWecom: 'No WeCom bots configured',
      emptyQQ: 'No QQ bots configured',
      emptyFeishu: 'No Feishu bots configured',
      emptyTelegram: 'No Telegram bots configured',
      tabs: {
        strategy: 'Strategy & Automation',
        channels: 'Channels & Alerts',
        templates: 'Message Templates',
        email: 'Email Settings'
      },
      sections: {
        basic: {
          title: 'Basic Settings',
          description: 'Configure basic system operation parameters.',
          refreshInterval: 'Data Refresh Interval',
          refreshIntervalHelp: 'Set the interval for automatically fetching upstream site data. Minimum 60 seconds.',
          seconds: 'Seconds'
        },
        thresholds: {
          title: 'Site Warning Thresholds',
          description: 'Configure default alarm triggers for all upstream sites.',
          balanceWarning: 'Balance Warning',
          balanceWarningHelp: 'Send an alert when an upstream site\'s balance (converted to CNY via recharge rate) falls below the configured amount.',
          multiplierChangeWarning: 'Multiplier Change Alert',
          multiplierChangeWarningHelp: 'Send notifications when any mapped group multiplier changes.'
        },
        pricing: {
          title: 'Auto-Pricing',
          description: 'Configure auto-adjustment rules when an upstream mapped group multiplier increases.',
          enableAutoPricing: 'Auto-Pricing',
          enableAutoPricingHelp: 'Automatically adjust "Own Group" multiplier when an upstream group multiplier increases.',
          strategy: 'Adjustment Strategy',
          strategyFixed: 'Fixed Increase (+)',
          strategyPercentage: 'Percentage Increase (%)',
          fixedValuePlaceholder: 'e.g., 0.1',
          percentageValuePlaceholder: 'e.g., 10'
        },
        channels: {
          title: 'Notification Channels',
          description: 'Configure third-party channels for receiving system alerts (e.g., DingTalk, WeCom, QQ, Telegram, Feishu).',
          dingtalk: 'DingTalk Bot',
          dingtalkHelp: 'Configure Webhook and Secret for DingTalk group bot.',
          wecom: 'WeCom Bot',
          wecomHelp: 'Configure the Webhook for a WeCom group bot. No signing secret is required.',
          qq: 'QQ Bot',
          qqHelp: 'Configure the app credentials and recipient user OpenID for an official QQ bot.',
          feishu: 'Feishu Bot',
          feishuHelp: 'Configure Webhook and Secret for Feishu group bot.',
          telegram: 'Telegram Bot',
          telegramHelp: 'Configure Bot Token and Chat ID for Telegram.',
          webhookUrl: 'Webhook URL',
          secret: 'Secret',
          appId: 'AppID',
          appIdPlaceholder: 'Enter the bot AppID',
          appSecret: 'AppSecret',
          appSecretPlaceholder: 'Enter the bot AppSecret',
          userOpenId: 'User OpenID',
          userOpenIdPlaceholder: 'Enter the recipient user openid',
          userOpenIdHelp: 'Add the user to the message-list sandbox, have them scan and message the bot, then read it from the C2C_MESSAGE_CREATE event.',
          botToken: 'Bot Token',
          chatId: 'Chat ID',
          proxyUrl: 'Proxy URL (Optional)',
          proxyUrlPlaceholder: 'e.g. http://127.0.0.1:7890',
          proxyUrlHelp: 'Use a proxy when the server cannot reach Telegram directly. Leave blank for direct connection.',
          loading: 'Loading notification channel settings...',
          testConnection: 'Test Connection',
          testConnectionSuccess: 'Sent Successfully'
        },
        templates: {
          balanceDefaultTemplate: '🔴 **Balance warning**\n\n🏷️ **Site:** {siteName}\n💰 **Current balance:** CNY {balance}\n⚠️ **Warning threshold:** CNY {threshold}\n\nPlease review and recharge the upstream account to avoid service interruption.',
          multiplierDefaultTemplate: '🟠 **Multiplier change warning**\n\n🏷️ **Site:** {siteName}\n📦 **Group:** {groupName}\n📊 **Rate:** {oldRate}x → **{newRate}x** ({changeDirection})\n\n🔎 Review the cost change and confirm whether downstream pricing needs adjustment.',
          balanceTemplatePlaceholder: 'e.g., 🔴 {siteName} has CNY {balance} remaining, below CNY {threshold}.',
          multiplierTemplatePlaceholder: 'e.g., 🟠 {siteName} / {groupName}: {oldRate}x → {newRate}x.'
        }
      },
      errors: {
        network: 'Network or CORS request failed. Check the API URL and cross-origin settings.',
        request: 'The notification channel test request failed. Try again later.',
        unknown: 'An unknown error occurred while testing the notification channel.',
        invalidChannel: 'The notification channel type is invalid.',
        missingWebhook: 'Enter the robot webhook URL first.',
        missingQQConfig: 'Enter the QQ bot AppID, AppSecret, and User OpenID first.',
        missingTelegramConfig: 'Enter the Telegram Bot Token and Chat ID first.',
        sendFailed: 'Failed to send the test message. Check the robot configuration and network connectivity.'
      },
      smtp: {
        title: 'SMTP Email Settings',
        description: 'Configure the SMTP server used to send system emails.',
        host: 'SMTP Host',
        port: 'Port',
        tlsMode: 'TLS Mode',
        tlsStarttls: 'STARTTLS (587)',
        tlsImplicit: 'Implicit TLS (465)',
        username: 'Username',
        password: 'Password',
        passwordConfigured: 'Password saved',
        passwordNotConfigured: 'No password saved',
        passwordKeepPlaceholder: 'Leave blank to keep the saved password',
        passwordNewPlaceholder: 'Enter SMTP password',
        fromEmail: 'From Email',
        fromName: 'From Name',
        testRecipient: 'Test Recipient',
        saveSuccess: 'SMTP settings saved',
        testEmail: 'Send Test Email',
        testEmailSuccess: 'Test email sent',
        dirtyBeforeTest: 'Save the current SMTP settings before sending a test email',
        errors: {
          validation: 'Check the SMTP settings.',
          missingConfig: 'Save the SMTP settings first.',
          invalidTlsMode: 'The TLS mode is invalid.',
          invalidEmail: 'The email address is invalid.',
          invalidPort: 'Port must be an integer between 1 and 65535.',
          encryptionKeyUnavailable: 'The server has no SMTP encryption key configured.',
          decryptFailed: 'Unable to read the saved SMTP password.',
          sendFailed: 'Failed to send the test email.',
          persistence: 'Failed to save the SMTP settings.'
        }
      },
      emailTemplates: {
        title: 'Email Templates',
        description: 'Create reusable HTML emails and send a test from every saved template.',
        library: 'Template library',
        editor: 'Template editor',
        add: 'New Template',
        builtIn: 'Built-in',
        loading: 'Loading email templates...',
        empty: 'No templates available',
        name: 'Template Name',
        subject: 'Email Subject',
        htmlBody: 'HTML Body',
        preview: 'Email Preview',
        code: 'View source',
        previewTitle: 'Sandboxed email template preview',
        save: 'Save Template',
        delete: 'Delete',
        test: 'Send Test Email',
        testRecipient: 'Test Recipient',
        testRecipientPlaceholder: "name{'@'}example.com",
        unsaved: 'You have unsaved changes',
        dirtyBeforeTest: 'Save the template before sending a test email.',
        discardConfirm: 'This template has unsaved changes. Discard them?',
        deleteConfirm: 'Delete “{name}”? This action cannot be undone.',
        newTemplateName: 'Custom Template',
        newTemplateSubject: 'Enter an email subject',
        newTemplateHtml: '<div style="font-family:Arial,sans-serif;padding:32px"><h1>Write your headline here</h1><p>Compose your email content here.</p></div>',
        createSuccess: 'Template created',
        saveSuccess: 'Template saved',
        deleteSuccess: 'Template deleted',
        testEmailSuccess: 'Test email sent',
        errors: {
          validation: 'Enter a name, a single-line subject, and HTML content no larger than 100KB.',
          invalidEmail: 'The test recipient email is invalid.',
          notFound: 'The email template does not exist or was deleted.',
          builtInProtected: 'Built-in templates cannot be deleted, but they remain editable.',
          limitReached: 'You can create up to 50 custom templates.',
          persistence: 'Failed to save the email template. Try again later.'
        }
      }
    },
    system: {
      version: 'Version {version}',
      openRelease: 'View release notes',
      openGithubRepository: 'View source on GitHub',
      errors: {
        network: 'System info request failed. Check your network connection.',
        request: 'System request failed. Please try again later.'
      }
    }
  },
  embed: {
    tickets: {
      page: {
        loading: 'Loading ticket system...'
      },
      list: {
        title: 'My Tickets',
        refresh: 'Refresh',
        create: 'New Ticket',
        loading: 'Loading tickets...',
        emptyTitle: 'No tickets yet',
        emptyDescription: 'Click "New Ticket" to submit your first question.'
      },
      createModal: {
        title: 'New Ticket'
      },
      form: {
        manualEmail: 'Contact Email',
        manualEmailPlaceholder: 'Enter the email to receive replies',
        title: 'Title',
        titlePlaceholder: 'Briefly describe your issue',
        body: 'Details',
        bodyPlaceholder: 'Describe the issue in detail',
        category: 'Category',
        categoryPlaceholder: 'Select a category',
        priority: 'Priority',
        priorityPlaceholder: 'Select a priority',
        submit: 'Submit Ticket',
        cancel: 'Cancel',
        images: 'Images',
        imagesCount: '{count} / {max}',
        addImage: 'Add Image',
        imagesHint: 'JPEG/PNG/WEBP/GIF only, up to 5MB per image.'
      },
      detail: {
        back: 'Back to list',
        loading: 'Loading ticket details...',
        support: 'Support',
        you: 'You',
        replyPlaceholder: 'Type your reply...',
        send: 'Send',
        closedNotice: 'This ticket is closed and can no longer be replied to.'
      },
      attachments: {
        loadFailed: 'Failed to load image'
      },
      status: {
        open: 'Open',
        pending: 'Pending',
        replied: 'Replied',
        closed: 'Closed'
      },
      errors: {
        network: 'Network or CORS request failed. Check the API URL and cross-origin settings.',
        request: 'The ticket request failed. Try again later.',
        unknown: 'An unknown error occurred. Please try again later.',
        missingParams: 'This page is missing required parameters and cannot start a session. If you opened it via the "Open Ticket Page" preview button, this is expected — open this page from within a real Sub2API iframe instead.',
        formIncomplete: 'Please fill in the contact email, title, and details, and select a category and priority.',
        configNotFound: 'The embed configuration does not exist.',
        disabled: 'The ticket feature has been disabled by the administrator.',
        invalidSrcHost: 'Invalid source host.',
        srcHostMismatch: 'The source host is not trusted.',
        sub2apiAuth: 'Identity verification failed. Please refresh the page and try again.',
        sub2apiRequest: 'Failed to fetch user info. Please try again later.',
        userMismatch: 'User identity verification failed.',
        sessionInvalid: 'Session expired. Please refresh the page and try again.',
        invalidEmail: 'Please enter a valid email address.',
        titleRequired: 'Please enter a title.',
        bodyRequired: 'Please enter the details.',
        categoryRequired: 'Please select a category.',
        priorityRequired: 'Please select a priority.',
        invalidCategory: 'The selected category is not part of the current ticket configuration. Please choose again.',
        invalidPriority: 'The selected priority is not part of the current ticket configuration. Please choose again.',
        ticketClosed: 'This ticket is closed and can no longer be replied to.',
        tooManyImages: 'The number of images exceeds the current limit.',
        invalidImageType: 'Only JPEG/PNG/WEBP/GIF images are supported.',
        imageTooLarge: 'Each image must be 5MB or smaller.',
        emptyImage: 'The image is empty, please choose another one.',
        attachmentLoadFailed: 'Failed to load image, please try again later.'
      }
    },
    leaderboard: {
      eyebrow: 'Usage ranking',
      title: 'Token Leaderboard',
      subtitle: 'Rank users by their actual token usage.',
      refresh: 'Refresh leaderboard',
      errorTitle: 'Unable to load leaderboard',
      emptyTitle: 'No usage in this period',
      emptyDescription: 'Choose another period or refresh again later.',
      anonymous: 'User {id}',
      podiumLabel: 'Top three users',
      updatedAt: 'Updated at {time}',
      period: { label: 'Reporting period', today: 'Today', '7d': '7 days', '30d': '30 days' },
      metrics: { tokens: 'Tokens', requests: 'Requests', cost: 'Actual spend' },
      table: { title: 'Full ranking', caption: '{count} users', rank: 'Rank', user: 'User' },
      errors: {
        network: 'Network request failed. Try again later.',
        request: 'The leaderboard request failed. Try again later.',
        missingParams: 'Required iframe session parameters are missing. Open this page from a Sub2API custom menu.',
        configNotFound: 'The leaderboard embed is not configured or has no source site.',
        invalidSrcHost: 'The source address is invalid.',
        srcHostMismatch: 'The current source site does not match the embed configuration.',
        sourceBinding: 'The leaderboard source has changed. Ask an administrator to save the embed settings again.',
        sub2apiAuth: 'Identity verification failed. Refresh the page and try again.',
        sub2apiRequest: 'Unable to connect to the source site. Try again later.',
        userMismatch: 'User identity verification failed.',
        sessionInvalid: 'The session expired. Refresh the page and try again.',
        adminSession: 'The admin session is unavailable. Ask the site administrator to reconnect the workspace.',
        upstreamUnsupported: 'This Sub2API version does not support the leaderboard. Ask an administrator to upgrade it.',
        upstreamRequest: 'Leaderboard data is temporarily unavailable. Try again later.'
      }
    },
    lottery: {
      eyebrow: 'Lottery entry',
      title: 'Lottery Campaigns',
      subtitle: 'Enter active campaigns, keep your receipt hash, and check your result after the draw.',
      page: {
        loading: 'Loading lottery campaigns...'
      },
      common: {
        empty: 'None',
        noDescription: 'No description provided.'
      },
      list: {
        title: 'Campaigns',
        count: '{count} items',
        loading: 'Loading campaigns...',
        empty: 'No public lottery campaigns are available.'
      },
      detail: {
        loading: 'Loading campaign details...',
        empty: 'Select a campaign to view details.'
      },
      sections: {
        schedule: 'Schedule',
        prizes: 'Prizes',
        winners: 'Public winners',
        integrity: 'Draw integrity',
        myEntry: 'My entry',
        myResult: 'My result',
        entries: 'Public entry register'
      },
      metrics: {
        entries: '{count} entries',
        winners: '{count} winners',
        winnersLabel: 'Winners'
      },
      actions: {
        refresh: 'Refresh',
        enter: 'Enter campaign',
        withdraw: 'Withdraw entry',
        browseCampaigns: 'Browse other campaigns',
        returnToDraw: 'Return to my draw',
        copyVoucher: 'Copy voucher'
      },
      transparency: {
        title: 'Public, verifiable draw',
        description: 'The entry register, receipt hashes, seed commitment, and draw snapshot are available for verification.',
        activeEntries: '{count} active entries',
        algorithmV2: 'v2 builds the snapshot from public entry IDs, masked emails, and receipt hashes, then applies a deterministic HMAC-SHA256 shuffle with the revealed seed.',
        algorithmLegacy: 'This campaign uses the compatible v1 algorithm. Its seed commitment and result remain verifiable, but the snapshot contains a legacy internal identifier.'
      },
      countdown: {
        opensIn: 'Registration opens in',
        closesIn: 'Registration closes in',
        drawsIn: 'Draw starts in',
        drawTime: 'Draw time',
        noTimer: 'Timer',
        days: '{days}d {hours}h {minutes}m {seconds}s',
        hours: '{hours}h {minutes}m {seconds}s',
        minutes: '{minutes}m {seconds}s',
        seconds: '{seconds}s'
      },
      drawReveal: {
        ariaLabel: 'Lottery result reveal',
        countdown: {
          eyebrow: 'Draw countdown',
          description: 'The entry snapshot is locked and ready for the draw.'
        },
        drawing: {
          eyebrow: 'Verifiable draw',
          title: 'Selecting the lucky entries',
          description: 'Calculating this draw with the published algorithm...'
        },
        won: {
          eyebrow: 'Congratulations',
          title: 'You won {prize}',
          description: 'Your win is confirmed. Reward delivery is available on the result page.'
        },
        lost: {
          eyebrow: 'Draw result',
          title: 'Not selected this time',
          description: 'Thank you for taking part. We hope to see you in the next draw.'
        },
        spectator: {
          eyebrow: 'Draw complete',
          title: 'The results are in',
          description: 'The public winners and draw integrity details have been updated.'
        },
        viewResult: 'View draw result'
      },
      status: {
        draft: 'Draft',
        scheduled: 'Scheduled',
        open: 'Open',
        closed: 'Registration closed',
        drawing: 'Drawing',
        drawn: 'Drawn',
        fulfilling: 'Fulfilling',
        completed: 'Completed',
        partial: 'Partial',
        cancelled: 'Cancelled'
      },
      drawMode: {
        manual: 'Manual draw',
        scheduled: 'Scheduled draw'
      },
      prizeType: {
        balance: 'Balance',
        subscription: 'Subscription'
      },
      deliveryMode: {
        sub2api_auto: 'Automatic Sub2API delivery',
        voucher: 'Voucher delivered after winning',
        manual: 'Manual redemption with organizer'
      },
      entryStatus: {
        active: 'Active',
        withdrawn: 'Withdrawn'
      },
      rewardStatus: {
        pending: 'Pending',
        processing: 'Processing',
        fulfilled: 'Fulfilled',
        retryable_failed: 'Retryable failed',
        manual_attention: 'Manual attention',
        failed: 'Failed'
      },
      fields: {
        algorithmVersion: 'Algorithm version',
        drawAt: 'Draw at',
        deliveryMode: 'Delivery method',
        enteredAt: 'Entered at',
        entrySnapshotHash: 'Entry snapshot hash',
        entryId: 'Entry ID',
        entryStatus: 'Entry status',
        prize: 'Prize',
        prizeSlot: 'Prize slot',
        receiptHash: 'Receipt hash',
        registrationEnd: 'Registration end',
        registrationStart: 'Registration start',
        revealedSeed: 'Revealed seed',
        rewardMessage: 'Reward message',
        rewardStatus: 'Reward status',
        seedCommitment: 'Seed commitment',
        voucherCode: 'My voucher',
        manualContact: 'Redemption contact'
      },
      prizes: {
        empty: 'No prizes have been published for this campaign.',
        quantity: 'x {count}',
        balanceValue: 'Balance amount: {amount}',
        subscriptionValue: 'Group: {group}, reward multiplier: {multiplier}, valid for {days} days'
      },
      winners: {
        private: 'Winner visibility is controlled by the campaign owner.',
        empty: 'No public winners are available yet.',
        count: '{count} public winners',
        row: '{email} won {prize} #{slot}'
      },
      entries: {
        description: 'Emails are masked. Each receipt hash can be checked against the registration and draw snapshot.',
        count: '{active} active / {total} records',
        empty: 'No entry records yet.'
      },
      entry: {
        none: 'You have not entered this campaign.'
      },
      result: {
        none: {
          title: 'Not entered',
          description: 'Enter while registration is open to receive a receipt hash.'
        },
        pending: {
          title: 'Result pending',
          description: 'Your entry is active. Refresh after the draw to see the result.'
        },
        won: {
          title: 'You won',
          description: 'Your winning prize and reward delivery status are shown below.'
        },
        lost: {
          title: 'Not selected',
          description: 'The draw has completed and this entry was not selected.'
        },
        withdrawn: {
          title: 'Entry withdrawn',
          description: 'This withdrawn entry is final for the current campaign.'
        }
      },
      errors: {
        title: 'Unable to load lottery',
        network: 'Network request failed. Check the connection and try again.',
        request: 'The lottery request failed. Try again later.',
        missingParams: 'Required iframe session parameters are missing. Open this page from a Sub2API custom menu.',
        configNotFound: 'The lottery embed is not configured or has no source site.',
        invalidSrcHost: 'The source address is invalid.',
        srcHostMismatch: 'The current source site does not match the embed configuration.',
        sub2apiAuth: 'Identity verification failed. Refresh the page and try again.',
        sub2apiRequest: 'Unable to connect to the source site. Try again later.',
        userMismatch: 'User identity verification failed.',
        userInactive: 'This source account is inactive.',
        sessionInvalid: 'The session expired. Refresh the page and try again.',
        adminSession: 'The admin session is unavailable. Ask the site administrator to reconnect the workspace.',
        sourceBinding: 'The lottery source has changed. Ask an administrator to save the embed settings again.',
        campaignNotOpen: 'This campaign is not open for entries.',
        entryNotFound: 'No active entry was found for this campaign.',
        upstreamRequest: 'Source site data is temporarily unavailable. Try again later.',
        alreadyEntered: 'You already have an entry for this campaign.',
        copy: 'Copy failed. Select the voucher manually.'
      }
    }
  }
}
