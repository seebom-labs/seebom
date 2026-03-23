import { Component, OnInit, ChangeDetectionStrategy, ChangeDetectorRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ScrollingModule } from '@angular/cdk/scrolling';
import { ApiService } from '../../core/api.service';
import { DependencyStatsResponse, DependencyStatsItem } from '../../core/api.models';

type SortField = 'projects' | 'name' | 'versions' | 'vulns';

@Component({
  selector: 'app-dependency-stats',
  standalone: true,
  imports: [CommonModule, ScrollingModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="deps-page">
      <h1>Dependency Statistics</h1>
      <p class="subtitle">Most-used dependencies across all projects.</p>

      <div class="summary" *ngIf="stats">
        <div class="summary-card">
          <span class="big-number">{{ stats.total_unique_deps | number }}</span>
          <span class="label">Unique Dependencies</span>
        </div>
      </div>

      <div class="table-header" *ngIf="stats">
        <span class="col-rank">#</span>
        <span class="col-name sortable" (click)="toggleSort('name')" [class.active]="sortField === 'name'">
          Package {{ sortField === 'name' ? (sortAsc ? '↑' : '↓') : '' }}
        </span>
        <span class="col-projects sortable" (click)="toggleSort('projects')" [class.active]="sortField === 'projects'">
          Projects {{ sortField === 'projects' ? (sortAsc ? '↑' : '↓') : '' }}
        </span>
        <span class="col-versions sortable" (click)="toggleSort('versions')" [class.active]="sortField === 'versions'">
          Versions {{ sortField === 'versions' ? (sortAsc ? '↑' : '↓') : '' }}
        </span>
        <span class="col-vulns sortable" (click)="toggleSort('vulns')" [class.active]="sortField === 'vulns'">
          Vulns {{ sortField === 'vulns' ? (sortAsc ? '↑' : '↓') : '' }}
        </span>
      </div>

      <cdk-virtual-scroll-viewport itemSize="56" class="viewport" *ngIf="stats">
        <div *cdkVirtualFor="let dep of sortedDeps; let i = index; trackBy: trackBy" class="dep-row">
          <span class="col-rank">{{ i + 1 }}</span>
          <div class="col-name">
            <span class="pkg-name">{{ dep.package_name }}</span>
            <span class="pkg-purl" *ngIf="dep.purl">{{ dep.purl }}</span>
          </div>
          <span class="col-projects">
            <strong>{{ dep.project_count | number }}</strong>
          </span>
          <span class="col-versions">
            <span class="version-pill" *ngFor="let v of dep.versions.slice(0, 5)">{{ v }}</span>
            <span class="more" *ngIf="dep.versions.length > 5">+{{ dep.versions.length - 5 }}</span>
          </span>
          <span class="col-vulns" [class.has-vulns]="dep.vuln_count > 0">
            {{ dep.vuln_count | number }}
          </span>
        </div>
      </cdk-virtual-scroll-viewport>

      <p *ngIf="!stats" class="loading">Loading dependency statistics...</p>
    </div>
  `,
  styles: [`
    .deps-page { padding: 24px; height: 100%; display: flex; flex-direction: column; }
    h1 { margin: 0; font-size: 1.1rem; font-weight: 700; letter-spacing: -0.02em; }
    .subtitle { color: var(--text-secondary); margin: 4px 0 16px; font-size: 0.8rem; }
    .summary { margin-bottom: 16px; }
    .summary-card {
      display: inline-flex; flex-direction: column; align-items: center;
      background: var(--surface-alt); padding: 14px 28px; border-radius: 4px;
      border: 1px solid var(--border);
    }
    .big-number { font-size: 1.5rem; font-weight: 700; color: var(--text); letter-spacing: -0.02em; }
    .label { font-size: 0.7rem; color: var(--text-secondary); text-transform: uppercase; letter-spacing: 0.04em; }
    .table-header {
      display: flex; align-items: center; gap: 16px; padding: 6px 12px;
      background: var(--bg); border-radius: 2px; font-weight: 600; font-size: 0.7rem;
      color: var(--text-secondary); text-transform: uppercase; letter-spacing: 0.04em; margin-bottom: 2px;
    }
    .sortable {
      cursor: pointer; user-select: none; transition: color 0.15s;
    }
    .sortable:hover { color: var(--accent); }
    .sortable.active { color: var(--accent); }
    .viewport { flex: 1; min-height: 400px; }
    .dep-row {
      height: 52px; display: flex; align-items: center; gap: 16px;
      padding: 0 12px; border-bottom: 1px solid var(--border);
    }
    .col-rank { width: 36px; color: var(--text-muted); font-size: 0.8rem; text-align: center; font-variant-numeric: tabular-nums; }
    .col-name { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
    .pkg-name { font-weight: 600; font-size: 0.85rem; }
    .pkg-purl { color: var(--text-muted); font-size: 0.68rem; overflow: hidden; text-overflow: ellipsis; }
    .col-projects { width: 70px; text-align: center; }
    .col-versions { width: 240px; display: flex; gap: 3px; flex-wrap: wrap; align-items: center; }
    .version-pill {
      background: var(--bg); color: var(--text-secondary); padding: 1px 5px; border-radius: 2px;
      font-size: 0.68rem; white-space: nowrap;
    }
    .more { font-size: 0.68rem; color: var(--text-muted); }
    .col-vulns { width: 56px; text-align: center; font-weight: 600; font-variant-numeric: tabular-nums; }
    .has-vulns { color: var(--severity-critical); background: var(--severity-critical-bg); padding: 2px 6px; border-radius: 2px; }
    .loading { color: var(--text-muted); }
  `],
})
export class DependencyStatsComponent implements OnInit {
  stats: DependencyStatsResponse | null = null;
  sortedDeps: DependencyStatsItem[] = [];
  sortField: SortField = 'projects';
  sortAsc = false;

  constructor(
    private readonly api: ApiService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngOnInit(): void {
    this.api.getDependencyStats(100).subscribe((data) => {
      this.stats = data;
      this.applySort();
      this.cdr.markForCheck();
    });
  }

  toggleSort(field: SortField): void {
    if (this.sortField === field) {
      this.sortAsc = !this.sortAsc;
    } else {
      this.sortField = field;
      this.sortAsc = field === 'name';
    }
    this.applySort();
    this.cdr.markForCheck();
  }

  private applySort(): void {
    if (!this.stats) return;
    const dir = this.sortAsc ? 1 : -1;
    this.sortedDeps = [...this.stats.top_dependencies].sort((a, b) => {
      switch (this.sortField) {
        case 'projects':
          return (a.project_count - b.project_count) * dir;
        case 'name':
          return a.package_name.localeCompare(b.package_name) * dir;
        case 'versions':
          return (a.versions.length - b.versions.length) * dir;
        case 'vulns':
          return (a.vuln_count - b.vuln_count) * dir;
        default:
          return 0;
      }
    });
  }

  trackBy(_i: number, dep: DependencyStatsItem): string { return dep.package_name + dep.purl; }
}
