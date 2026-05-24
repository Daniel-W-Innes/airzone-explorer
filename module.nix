{ airzonePackage }:
{
  config,
  lib,
  ...
}:
let
  cfg = config.services.airzone-explorer;

  mkFlag = name: value: "${name}=${toString value}";

  cliArgs =
    [
      (mkFlag "--email" cfg.email)
      (mkFlag "--password-file" cfg.passwordFile)
      (mkFlag "--base-url" cfg.baseURL)
      (mkFlag "--listen-host" cfg.host)
      (mkFlag "--listen-port" cfg.port)
      (mkFlag "--metrics-path" cfg.metricsPath)
      (mkFlag "--timeout" cfg.timeout)
    ]
    ++ cfg.extraFlags;
in
{
  options.services.airzone-explorer = {
    enable = lib.mkEnableOption "the Airzone Prometheus exporter";

    package = lib.mkOption {
      type = lib.types.package;
      default = airzonePackage;
      defaultText = lib.literalExpression "inputs.airzone-explorer.packages.\${pkgs.stdenv.hostPlatform.system}.default";
      description = "Package providing the `airzone-explorer` binary.";
    };

    email = lib.mkOption {
      type = lib.types.str;
      example = "me@example.com";
      description = "Airzone account email passed to `--email`.";
    };

    passwordFile = lib.mkOption {
      type = lib.types.str;
      example = "/run/secrets/airzone-password";
      description = "Path to a file containing the Airzone account password, passed to `--password-file`.";
    };

    baseURL = lib.mkOption {
      type = lib.types.str;
      default = "https://m.airzonecloud.com/api/v1";
      description = "Airzone API base URL passed to `--base-url`.";
    };

    host = lib.mkOption {
      type = lib.types.str;
      default = "";
      example = "127.0.0.1";
      description = "HTTP listen host passed to `--listen-host`. Leave empty to bind all interfaces.";
    };

    port = lib.mkOption {
      type = lib.types.port;
      default = 9922;
      description = "HTTP listen port passed to `--listen-port`.";
    };

    metricsPath = lib.mkOption {
      type = lib.types.str;
      default = "/metrics";
      description = "HTTP metrics path passed to `--metrics-path`.";
    };

    timeout = lib.mkOption {
      type = lib.types.str;
      default = "15s";
      example = "30s";
      description = "HTTP timeout passed to `--timeout`.";
    };

    extraFlags = lib.mkOption {
      type = with lib.types; listOf str;
      default = [ ];
      example = [ "--log-level=debug" ];
      description = "Additional command-line flags appended to the service command.";
    };

    user = lib.mkOption {
      type = lib.types.str;
      default = "airzone-explorer";
      description = "User account under which the exporter runs.";
    };

    group = lib.mkOption {
      type = lib.types.str;
      default = "airzone-explorer";
      description = "Group under which the exporter runs.";
    };
  };

  config = lib.mkIf cfg.enable {
    users.groups = lib.mkIf (cfg.group == "airzone-explorer") {
      airzone-explorer = { };
    };

    users.users = lib.mkIf (cfg.user == "airzone-explorer") {
      airzone-explorer = {
        isSystemUser = true;
        group = cfg.group;
      };
    };

    systemd.services.airzone-explorer = {
      description = "Airzone Prometheus exporter";
      after = [ "network-online.target" ];
      wants = [ "network-online.target" ];
      wantedBy = [ "multi-user.target" ];

      serviceConfig = {
        ExecStart = lib.concatStringsSep " " ([ (lib.getExe cfg.package) ] ++ map lib.escapeShellArg cliArgs);
        Restart = "on-failure";
        RestartSec = 5;
        User = cfg.user;
        Group = cfg.group;
        NoNewPrivileges = true;
        PrivateTmp = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        ProtectKernelTunables = true;
        ProtectKernelModules = true;
        ProtectControlGroups = true;
        LockPersonality = true;
        RestrictRealtime = true;
        RestrictSUIDSGID = true;
        MemoryDenyWriteExecute = true;
        SystemCallArchitectures = "native";
      };
    };
  };
}
