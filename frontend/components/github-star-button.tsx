import { Github, Star } from 'lucide-react'

async function getStarCount(): Promise<number | null> {
    try {
        const res = await fetch('https://api.github.com/repos/aynaash/NextDeploy', {
            next: { revalidate: 3600 }, // cache for 1 hour
        })
        if (!res.ok) return null
        const data = await res.json()
        return data.stargazers_count ?? null
    } catch {
        return null
    }
}

export default async function GitHubStarButton() {
    const stars = await getStarCount()

    return (
        <a
            href="https://github.com/aynaash/NextDeploy"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-surface-800 hover:bg-surface-700 border border-surface-700 hover:border-surface-600 text-white text-sm font-medium transition-all duration-200 group"
        >
            <Github className="w-4 h-4 text-muted-400 group-hover:text-white transition-colors" />
            <span>Star on GitHub</span>
            {stars !== null && (
                <>
                    <span className="w-px h-4 bg-surface-600" />
                    <span className="flex items-center gap-1 text-highlight-400 font-semibold">
                        <Star className="w-3.5 h-3.5 fill-highlight-400" />
                        {stars >= 1000 ? `${(stars / 1000).toFixed(1)}k` : stars}
                    </span>
                </>
            )}
        </a>
    )
}
