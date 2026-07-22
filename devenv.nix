{ pkgs, ... }:

{
  dotenv.enable = true;

  languages.go = {
    enable = true;
    version = "1.26.5";

    lsp.enable = true;
  };

  packages = [
    pkgs.gnumake
    pkgs.gcc
    pkgs.git

    pkgs.golangci-lint

    pkgs.postgresql_16
    pkgs.redis
  ];

  services.postgres = {
    enable = true;
    package = pkgs.postgresql_16;

    listen_addresses = "127.0.0.1";
    port = 5432;

    initialDatabases = [
      {
        name = "goat";
        user = "postgres";
        pass = "postgres";
      }
    ];
  };

  services.redis = {
    enable = true;
    port = 6380;
  };

  scripts.check.exec = ''
    make verify
    make test
    make lint
  '';

  processes.goat-api = {
    exec = ''
      make run
    '';
    after = [
      "devenv:processes:postgres"
      "devenv:processes:redis"
    ];
  };

  enterShell = ''
    echo "GOAT API development environment loaded"
    echo "---------------------------------------------"
    echo "PostgreSQL: 127.0.0.1:5432"
    echo "Redis:      127.0.0.1:6380"
    echo "Go:         $(go version)"
    echo "gopls:      $(command -v gopls)"
    echo "Linter:     $(command -v golangci-lint)"
    echo "---------------------------------------------"
  '';
}
