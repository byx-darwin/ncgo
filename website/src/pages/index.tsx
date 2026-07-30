import React from 'react';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Translate from '@docusaurus/Translate';
import Terminal from '../components/Terminal';
import FileTree from '../components/FileTree';
import CommandTabs from '../components/CommandTabs';
import CopyCommand from '../components/CopyCommand';
import Reveal from '../components/Reveal';
import { copyFor, heroTerminal, heroTree, commandTabsData, infraBadges, INSTALL_CMD } from '../data/copy';

export default function Home(): React.JSX.Element {
  const { i18n } = useDocusaurusContext();
  const c = copyFor(i18n.currentLocale);

  return (
    <Layout>
      <main className="landing">
        {/* Hero：左文案 + 右活终端/目录树（非居中三件套） */}
        <section className="hero-split">
          <div className="hero-copy">
            <span className="hero-kicker">{c.heroKicker}</span>
            <h1 className="hero-title">{c.heroTitle}</h1>
            <p className="hero-sub">{c.heroSub}</p>
            <CopyCommand command={INSTALL_CMD} />
            <div className="hero-ctas">
              <Link className="btn btn-primary" to="/docs/intro">{c.heroCtaDocs}</Link>
              <Link className="btn btn-ghost" to="https://github.com/byx-darwin/ncgo">{c.heroCtaGithub}</Link>
            </div>
          </div>
          <div className="hero-visual">
            <Terminal lines={heroTerminal} title="ncgo — scaffold" />
            <FileTree rows={heroTree} className="hero-tree" />
          </div>
        </section>

        {/* 工作流横轴 */}
        <section className="section">
          <Reveal><h2 className="section-heading">{c.workflowHeading}</h2></Reveal>
          <ol className="workflow">
            {c.workflow.map((s, i) => (
              <Reveal key={s.title} delay={i * 90}>
                <li className="workflow-step">
                  <span className="workflow-num">{String(i + 1).padStart(2, '0')}</span>
                  <h3>{s.title}</h3>
                  <p>{s.body}</p>
                </li>
              </Reveal>
            ))}
          </ol>
        </section>

        {/* 不对称 Bento 特性 */}
        <section className="section">
          <Reveal><h2 className="section-heading">{c.featuresHeading}</h2></Reveal>
          <div className="bento">
            {c.features.map((f, i) => (
              <Reveal key={f.title} delay={i * 70} className={f.big ? 'bento-big' : ''}>
                <div className="bento-card">
                  <h3>{f.title}</h3>
                  <p>{f.body}</p>
                </div>
              </Reveal>
            ))}
          </div>
        </section>

        {/* 基础设施集成带（横向滚动 marquee） */}
        <section className="section infra-band" aria-label="Supported infrastructure">
          <div className="marquee">
            <div className="marquee-track">
              {[...infraBadges, ...infraBadges].map((b, i) => (
                <span key={`${b}-${i}`} className="infra-badge">{b}</span>
              ))}
            </div>
          </div>
        </section>

        {/* 命令速览 Tabs */}
        <section className="section section-narrow">
          <Reveal><h2 className="section-heading">{c.commandsHeading}</h2></Reveal>
          <Reveal delay={100}><CommandTabs tabs={commandTabsData} /></Reveal>
        </section>

        {/* 安装 CTA */}
        <section className="section cta-band">
          <Reveal>
            <h2 className="cta-heading">{c.ctaHeading}</h2>
            <CopyCommand command={INSTALL_CMD} />
            <p className="cta-note">{c.footerNote}</p>
          </Reveal>
        </section>
      </main>
    </Layout>
  );
}
