import js from '@eslint/js'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import globals from 'globals'
import tseslint from 'typescript-eslint'

export default tseslint.config(
  { ignores: ['dist', 'src/components/ui', 'node_modules'] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      'no-useless-assignment': 'off',
      'preserve-caught-error': 'off',
      'react-hooks/set-state-in-effect': 'off',
      'react-refresh/only-export-components': [
        'warn',
        { allowConstantExport: true },
      ],
    },
  },
  {
    // XSS guard: wiki content is user-controlled, so HTML must only reach the
    // DOM through the sanitized Markdown pipeline (features/preview). A
    // justified exception needs an eslint-disable-next-line comment saying why
    // the input is safe.
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['**/*.test.{ts,tsx}', 'src/test-setup.ts'],
    rules: {
      'no-eval': 'error',
      'no-implied-eval': 'error',
      'no-new-func': 'error',
      'no-restricted-syntax': [
        'error',
        {
          selector: "JSXAttribute[name.name='dangerouslySetInnerHTML']",
          message:
            'dangerouslySetInnerHTML bypasses sanitization. Render through the Markdown pipeline or React elements.',
        },
        {
          selector: "JSXAttribute[name.name='srcDoc']",
          message: 'srcDoc injects raw HTML into an iframe.',
        },
        {
          selector:
            'AssignmentExpression[left.type="MemberExpression"][left.property.name=/^(innerHTML|outerHTML)$/]',
          message:
            'Assigning innerHTML/outerHTML bypasses sanitization. Use textContent or React elements.',
        },
        {
          selector: "CallExpression[callee.property.name='insertAdjacentHTML']",
          message: 'insertAdjacentHTML bypasses sanitization.',
        },
        {
          selector:
            "CallExpression[callee.object.name='document'][callee.property.name=/^(write|writeln)$/]",
          message: 'document.write bypasses sanitization.',
        },
      ],
    },
  },
)
