import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';
import {execSync} from 'child_process';

// Get version from latest git tag
function getVersion(): string {
  try {
    const tag = execSync('git describe --tags --abbrev=0', {encoding: 'utf-8'}).trim();
    return tag;
  } catch {
    return 'dev';
  }
}

const config: Config = {
  title: 'Drift',
  tagline: 'Cross-platform mobile UI framework for Go',
  favicon: 'img/favicon.ico',

  customFields: {
    version: getVersion(),
  },

  // Deployment config
  url: 'https://driftframework.dev',
  baseUrl: '/',
  organizationName: 'go-drift',
  projectName: 'drift',
  trailingSlash: false,

  onBrokenLinks: 'throw',
  onBrokenAnchors: 'warn',

  markdown: {
    parseFrontMatter: async (params) => {
      const result = await params.defaultParseFrontMatter(params);
      return result;
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/go-drift/drift/tree/master/website/',
          // Enable versioning
          lastVersion: 'current',
          versions: {
            current: {
              label: 'Next',
              path: '',
            },
          },
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themes: [
    [
      '@easyops-cn/docusaurus-search-local',
      {
        hashed: true,
        language: ['en'],
        highlightSearchTermsOnTargetPage: true,
        explicitSearchResultPath: true,
      },
    ],
  ],

  themeConfig: {
    colorMode: {
      defaultMode: 'dark',
      respectPrefersColorScheme: true,
    },
    image: 'img/drift-social-card.png',
    navbar: {
      title: '',
      logo: {
        alt: 'Drift Logo',
        src: 'img/logo.svg',
        srcDark: 'img/logo-dark.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          type: 'docsVersionDropdown',
          position: 'right',
          dropdownActiveClassDisabled: true,
        },
        {
          href: 'https://github.com/go-drift/drift',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Docs',
          items: [
            {
              label: 'Getting Started',
              to: '/docs/intro',
            },
            {
              label: 'API Reference',
              to: '/docs/api/core',
            },
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'GitHub Discussions',
              href: 'https://github.com/go-drift/drift/discussions',
            },
            {
              label: 'Issues',
              href: 'https://github.com/go-drift/drift/issues',
            },
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/go-drift/drift',
            },
          ],
        },
      ],
      copyright: `Copyright ${new Date().getFullYear()} Drift Contributors. Built with Docusaurus.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['go', 'bash', 'yaml'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
