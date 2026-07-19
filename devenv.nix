{ pkgs, config, lib, ... }:

{
  dotenv.enable = true;

  languages.go = {
    enable = true;
    package = pkgs.go_1_25;
  };

  packages = with pkgs; [
    gnumake
    gcc
    golangci-lint
    postgresql_16
    redis
  ];

  services.postgres = {
    enable = true;
    package = pkgs.postgresql_16;

    # Matches your .env DB_NAME
    initialDatabases = [{ name = "goat"; }];

    # Matches your .env DB_USER and DB_PASSWORD
    initialScript = ''
      CREATE USER postgres WITH SUPERUSER PASSWORD 'postgres';
      ALTER DATABASE goat OWNER TO postgres;
    '';

    listen_addresses = "127.0.0.1";
    port = 5432;
  };

  services.redis = {
    enable = true;
    port = 6379;
  };

  processes.goat-api.exec = "make run";

  # 8. Shell Hook
  enterShell = ''
    echo "GOAT API Development Environment Loaded!"
    echo "-----------------------------------------"
    echo "DATABASE: localhost:5432"
    echo "REDIS:    localhost:6379"
    echo "GO:       $(go version)"
    echo "-----------------------------------------"
  '';
}
