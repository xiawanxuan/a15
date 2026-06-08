#!/bin/bash

APP_NAME="astro-scheduler"
SERVICE_NAME="astro-scheduler"
LOG_FILE="/var/log/astro-scheduler.log"

case "$1" in
    start)
        echo "Starting $APP_NAME..."
        sudo systemctl start $SERVICE_NAME
        ;;
    stop)
        echo "Stopping $APP_NAME..."
        sudo systemctl stop $SERVICE_NAME
        ;;
    restart)
        echo "Restarting $APP_NAME..."
        sudo systemctl restart $SERVICE_NAME
        ;;
    status)
        sudo systemctl status $SERVICE_NAME
        ;;
    logs)
        tail -f $LOG_FILE
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status|logs}"
        exit 1
        ;;
esac
