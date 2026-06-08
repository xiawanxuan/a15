#!/bin/bash

set -e

APP_NAME="astro-scheduler"
APP_DIR="/opt/astro-scheduler"
BINARY_NAME="astro-scheduler"
SERVICE_NAME="astro-scheduler"
CONFIG_FILE="configs/config.yaml"
LOG_FILE="/var/log/astro-scheduler.log"
USER="astro"

echo "Building $APP_NAME..."

go build -o $BINARY_NAME ./cmd/server

echo "Creating application directory..."

sudo mkdir -p $APP_DIR/configs
sudo mkdir -p $APP_DIR/data/archive

echo "Installing binary..."

sudo cp $BINARY_NAME $APP_DIR/
sudo cp $CONFIG_FILE $APP_DIR/configs/

echo "Creating service user..."

if ! id -u $USER &>/dev/null; then
    sudo useradd -r -s /bin/false $USER
fi

sudo chown -R $USER:$USER $APP_DIR

echo "Creating systemd service..."

sudo tee /etc/systemd/system/$SERVICE_NAME.service > /dev/null <<EOF
[Unit]
Description=Astro Scheduler Service
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/$BINARY_NAME --config $APP_DIR/configs/config.yaml
Restart=on-failure
RestartSec=10
StandardOutput=append:$LOG_FILE
StandardError=append:$LOG_FILE

[Install]
WantedBy=multi-user.target
EOF

echo "Reloading systemd..."

sudo systemctl daemon-reload
sudo systemctl enable $SERVICE_NAME

echo "Starting service..."

sudo systemctl start $SERVICE_NAME

echo "Service status:"
sudo systemctl status $SERVICE_NAME

echo ""
echo "Installation complete!"
echo "Config file: $APP_DIR/configs/config.yaml"
echo "Log file: $LOG_FILE"
echo "Service name: $SERVICE_NAME"
echo ""
echo "Useful commands:"
echo "  sudo systemctl start $SERVICE_NAME"
echo "  sudo systemctl stop $SERVICE_NAME"
echo "  sudo systemctl restart $SERVICE_NAME"
echo "  sudo systemctl status $SERVICE_NAME"
echo "  tail -f $LOG_FILE"
