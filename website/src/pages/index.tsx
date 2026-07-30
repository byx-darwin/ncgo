import React from 'react';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';

export default function Home() {
  return (
    <Layout title="Home" description="ncgo — AI-friendly scaffold CLI for Go microservices">
      <main style={{ padding: '4rem 1rem', textAlign: 'center' }}>
        <h1>ncgo</h1>
        <p>AI-friendly scaffold CLI for Go microservices.</p>
        <p>
          <Link to="/docs/intro">Get started →</Link>
        </p>
      </main>
    </Layout>
  );
}
