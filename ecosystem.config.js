module.exports = {
  apps: [
    {
      name: 'ipfs-api',
      script: './ipfs-api',
      exec_mode: 'fork',
      instances: 1,
      autorestart: true,
      watch: false,
      max_memory_restart: '1G',
      env: {
        NODE_ENV: 'production',
        PORT: 8085
      },
    },
  ],
};
