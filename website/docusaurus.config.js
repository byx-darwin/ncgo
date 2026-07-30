// @ts-check
const { themes } = require('prism-react-renderer');

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'ncgo',
  tagline: 'AI-friendly scaffold CLI for Go microservices',
  favicon: 'img/favicon.svg',

  url: 'https://byx-darwin.github.io',
  baseUrl: '/ncgo/',

  organizationName: 'byx-darwin',
  projectName: 'ncgo',

  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'throw',

  i18n: {
    defaultLocale: 'en',
    locales: ['en', 'zh-CN'],
    localeConfigs: {
      en: { label: 'English' },
      'zh-CN': { label: '简体中文' },
    },
  },

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          routeBasePath: 'docs',
          sidebarPath: require.resolve('./sidebars.js'),
          editUrl: 'https://github.com/byx-darwin/ncgo/tree/main/website/',
        },
        blog: false,
        theme: { customCss: require.resolve('./src/css/custom.css') },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      colorMode: { defaultMode: 'dark', disableSwitch: false, respectPrefersColorScheme: false },
      navbar: {
        title: 'ncgo',
        logo: { alt: 'ncgo logo', src: 'img/favicon.svg' },
        items: [
          { type: 'docSidebar', sidebarId: 'docs', position: 'left', label: 'Docs' },
          {
            href: 'https://github.com/byx-darwin/ncgo',
            label: 'GitHub',
            position: 'right',
          },
          { type: 'localeDropdown', position: 'right' },
        ],
      },
      footer: {
        style: 'dark',
        links: [
          {
            title: 'Docs',
            items: [{ label: 'Getting Started', to: '/docs/intro' }],
          },
          {
            title: 'Community',
            items: [
              { label: 'GitHub', href: 'https://github.com/byx-darwin/ncgo' },
              { label: 'Issues', href: 'https://github.com/byx-darwin/ncgo/issues' },
            ],
          },
        ],
        copyright: `Copyright © ${new Date().getFullYear()} ncgo authors. Built with Docusaurus.`,
      },
      prism: { theme: themes.github, darkTheme: themes.dracula },
    }),
};

module.exports = config;
