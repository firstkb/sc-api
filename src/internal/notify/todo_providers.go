package notify

// TODO:
// 1. Add SMTP/SES sender for email:
//    - use config (keys, region, sender address);
//    - support retry and error logging;
//    - support health-check.
//
// 2. Add SMS-provider (SNS/Twilio):
//    - config with tokens/sender number;
//    - normalize numbers to E.164;
//    - support rate-limit separate from OTP.
//
// 3. Switch Sender via notify.Config and pass it to notifysvc.
