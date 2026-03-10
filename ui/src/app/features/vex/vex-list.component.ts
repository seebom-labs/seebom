import { Component, OnInit, ChangeDetectionStrategy, ChangeDetectorRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterModule } from '@angular/router';
import { ScrollingModule } from '@angular/cdk/scrolling';
import { ApiService } from '../../core/api.service';
import { VEXStatementItem, VEXAffectedSBOM } from '../../core/api.models';

interface GroupedSBOM {
  project_name: string;
  versions: { sbom_id: string; label: string; document_name: string }[];
}

@Component({
  selector: 'app-vex-list',
  standalone: true,
  imports: [CommonModule, RouterModule, ScrollingModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="vex-list">
      <h1>VEX Statements</h1>
      <p class="subtitle">Vulnerability Exploitability eXchange – vendor assessments of vulnerability impact.</p>

      <cdk-virtual-scroll-viewport itemSize="96" class="viewport">
        <div *cdkVirtualFor="let stmt of statements; trackBy: trackByStmt" class="vex-row">
          <span class="status-badge" [class]="'status-' + stmt.status">
            {{ formatStatus(stmt.status) }}
          </span>
          <div class="vex-info">
            <div class="top-line">
              <a [routerLink]="['/cve-impact']" [queryParams]="{vuln: stmt.vuln_id}" class="vuln-id">{{ stmt.vuln_id }}</a>
              <span class="purl">{{ stmt.product_purl }}</span>
            </div>
            <div class="mid-line">
              <span class="justification" *ngIf="stmt.justification">
                {{ formatJustification(stmt.justification) }}
              </span>
              <span class="impact" *ngIf="stmt.impact_statement">
                {{ stmt.impact_statement }}
              </span>
              <span class="action" *ngIf="stmt.action_statement">
                Action: {{ stmt.action_statement }}
              </span>
            </div>
            <div class="sbom-line" *ngIf="stmt.affected_sboms?.length">
              <span class="sbom-label">Affected:</span>
              <ng-container *ngFor="let group of getGroupedSBOMs(stmt)">
                <span class="project-label">{{ group.project_name }}</span>
                <a *ngFor="let v of group.versions"
                   [routerLink]="['/sboms', v.sbom_id]"
                   class="version-tag"
                   [title]="v.document_name">
                  {{ v.label }}
                </a>
              </ng-container>
            </div>
            <div class="sbom-line none" *ngIf="!stmt.affected_sboms?.length">
              <span class="no-sboms">No matching SBOMs found</span>
            </div>
          </div>
          <span class="date">{{ stmt.vex_timestamp | date:'short' }}</span>
        </div>
      </cdk-virtual-scroll-viewport>

      <div *ngIf="loaded && statements.length === 0" class="empty-state">
        <div class="empty-icon">📋</div>
        <h2>No VEX Statements Found</h2>
        <p>VEX (Vulnerability Exploitability eXchange) statements provide vendor assessments of how vulnerabilities affect specific products.</p>
        <div class="how-to">
          <h3>How to add VEX data:</h3>
          <ol>
            <li>Create <code>.openvex.json</code> or <code>.vex.json</code> files following the <a href="https://openvex.dev" target="_blank">OpenVEX spec</a></li>
            <li>Place them alongside your SBOMs in the data directory</li>
            <li>Re-trigger ingestion: <code>make re-ingest</code></li>
          </ol>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .vex-list { padding: 24px; height: 100%; display: flex; flex-direction: column; }
    h1 { margin: 0; font-size: 1.1rem; font-weight: 700; letter-spacing: -0.02em; }
    .subtitle { color: var(--text-secondary); font-size: 0.8rem; margin: 4px 0 16px; }
    .viewport { flex: 1; min-height: 400px; }
    .vex-row {
      min-height: 92px; display: flex; align-items: center; gap: 14px;
      padding: 8px 12px; border-bottom: 1px solid var(--border);
    }
    .status-badge {
      padding: 3px 8px; border-radius: 2px; font-size: 0.65rem; font-weight: 600;
      text-transform: uppercase; min-width: 110px; text-align: center;
      letter-spacing: 0.03em; flex-shrink: 0;
    }
    .status-not_affected { background: var(--status-success-bg); color: var(--status-success); }
    .status-affected { background: var(--severity-critical-bg); color: var(--severity-critical); }
    .status-fixed { background: var(--status-info-bg); color: var(--accent-hover); }
    .status-under_investigation { background: var(--severity-high-bg); color: var(--status-warning); }
    .vex-info { flex: 1; display: flex; flex-direction: column; gap: 3px; overflow: hidden; }
    .top-line { display: flex; gap: 12px; align-items: center; }
    .vuln-id { font-weight: 600; font-size: 0.85rem; color: var(--accent); text-decoration: none; cursor: pointer; }
    .vuln-id:hover { text-decoration: underline; }
    .purl { color: var(--accent); font-size: 0.75rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .mid-line { display: flex; gap: 8px; font-size: 0.73rem; color: var(--text-secondary); overflow: hidden; }
    .justification { background: var(--bg); padding: 1px 5px; border-radius: 2px; white-space: nowrap; }
    .impact, .action { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .sbom-line { display: flex; align-items: center; gap: 5px; flex-wrap: wrap; }
    .sbom-line.none { opacity: 0.4; }
    .sbom-label { font-size: 0.68rem; color: var(--text-muted); margin-right: 2px; }
    .project-label {
      font-size: 0.68rem; font-weight: 600; color: var(--text-secondary);
      margin-left: 4px;
    }
    .project-label:first-of-type { margin-left: 0; }
    .version-tag {
      font-size: 0.65rem; color: var(--accent); text-decoration: none;
      background: var(--bg); padding: 1px 6px; border-radius: 2px;
      white-space: nowrap; font-family: monospace; font-weight: 500;
      border: 1px solid var(--border); transition: all 0.15s;
    }
    .version-tag:hover { background: var(--accent); color: #fff; border-color: var(--accent); }
    .no-sboms { font-size: 0.68rem; color: var(--text-muted); }
    .date { color: var(--text-muted); font-size: 0.75rem; min-width: 80px; flex-shrink: 0; }
    .empty-state {
      display: flex; flex-direction: column; align-items: center; justify-content: center;
      padding: 48px 24px; text-align: center; color: var(--text-secondary);
    }
    .empty-icon { font-size: 2.5rem; margin-bottom: 12px; }
    .empty-state h2 { margin: 0 0 8px; font-size: 1rem; font-weight: 600; color: var(--text); }
    .empty-state p { max-width: 500px; font-size: 0.85rem; line-height: 1.5; margin: 0 0 20px; }
    .how-to {
      text-align: left; background: var(--surface-alt); border: 1px solid var(--border);
      border-radius: 2px; padding: 16px 20px; max-width: 460px;
    }
    .how-to h3 { margin: 0 0 8px; font-size: 0.85rem; font-weight: 600; color: var(--text); }
    .how-to ol { margin: 0; padding-left: 20px; font-size: 0.8rem; line-height: 1.8; }
    .how-to code { background: var(--border); padding: 1px 4px; border-radius: 2px; font-size: 0.75rem; }
    .how-to a { color: var(--accent); text-decoration: none; }
  `],
})
export class VEXListComponent implements OnInit {
  statements: VEXStatementItem[] = [];
  loaded = false;

  private groupedCache = new Map<string, GroupedSBOM[]>();

  constructor(
    private readonly api: ApiService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngOnInit(): void {
    this.api.getVEXStatements(1, 5000).subscribe((response) => {
      this.statements = response.data;
      this.loaded = true;
      this.buildGroupedCache();
      this.cdr.markForCheck();
    });
  }

  getGroupedSBOMs(stmt: VEXStatementItem): GroupedSBOM[] {
    return this.groupedCache.get(stmt.vex_id) ?? [];
  }

  private buildGroupedCache(): void {
    for (const stmt of this.statements) {
      if (!stmt.affected_sboms?.length) continue;
      this.groupedCache.set(stmt.vex_id, this.groupSBOMs(stmt.affected_sboms));
    }
  }

  private groupSBOMs(sboms: VEXAffectedSBOM[]): GroupedSBOM[] {
    const map = new Map<string, GroupedSBOM>();

    for (const sbom of sboms) {
      const name = sbom.document_name || sbom.sbom_id;
      const projectKey = this.extractProjectKey(name);
      const versionLabel = this.extractVersionLabel(name);

      let group = map.get(projectKey);
      if (!group) {
        group = { project_name: projectKey, versions: [] };
        map.set(projectKey, group);
      }
      group.versions.push({
        sbom_id: sbom.sbom_id,
        label: versionLabel,
        document_name: name,
      });
    }

    // Sort versions descending within each group.
    for (const g of map.values()) {
      g.versions.sort((a, b) =>
        b.label.localeCompare(a.label, undefined, { numeric: true })
      );
    }

    return Array.from(map.values());
  }

  private extractProjectKey(name: string): string {
    return name
      .replace(/\s+v?\d+(\.\d+){1,}(-[\w.]+)?$/i, '')
      .replace(/\s+$/, '');
  }

  private extractVersionLabel(name: string): string {
    const match = name.match(/(v?\d+(\.\d+){1,}(-[\w.]+)?)$/i);
    return match ? match[1] : name;
  }

  formatStatus(status: string): string {
    return status.replace(/_/g, ' ');
  }

  formatJustification(justification: string): string {
    return justification.replace(/_/g, ' ');
  }

  trackByStmt(_index: number, item: VEXStatementItem): string {
    return item.vex_id;
  }
}
