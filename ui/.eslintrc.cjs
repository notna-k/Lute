module.exports = {
  root: true,
  env: { browser: true, es2020: true },
  extends: [
    'eslint:recommended',
    'plugin:@typescript-eslint/recommended',
    'plugin:react-hooks/recommended',
  ],
  ignorePatterns: ['dist', '.eslintrc.cjs'],
  parser: '@typescript-eslint/parser',
  plugins: ['react-refresh'],
  rules: {
    quotes: 'off',
    '@typescript-eslint/quotes': [
      'error',
      'single',
      { avoidEscape: true, allowTemplateLiterals: true },
    ],
    'jsx-quotes': ['error', 'prefer-single'],
    'react-refresh/only-export-components': [
      'warn',
      {
        allowConstantExport: true,
        allowExportNames: [
          'BUILD_STATE_LABEL',
          'MOD_KEY',
          'WORKER_STATE_LABEL',
          'authBridge',
          'isAppleOS',
          'stateLabel',
          'toastSubject',
          'useAuth',
          'useTheme',
          'useToast',
          'useUiPreferences',
        ],
      },
    ],
  },
}
