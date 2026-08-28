package setting

// CyberAutoBanEnabled controls whether users are disabled after reaching the
// configured number of upstream cyber-policy events. Events are still stored
// while this switch is off so enabling it later does not lose the audit trail.
var CyberAutoBanEnabled = false

// CyberAutoBanThreshold is the cumulative number of cyber-policy events that
// triggers an automatic user ban. Values below one disable the threshold.
var CyberAutoBanThreshold = 3
