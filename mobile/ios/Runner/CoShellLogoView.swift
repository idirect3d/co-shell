import UIKit

/// Renders the co-shell pixel-mosaic mascot (a little clam) as a grid of
/// accent-coloured cells, matching the co-shell web UI logo (FEATURE-371).
/// The art is a 12-row grid where "#" cells are filled and spaces are empty.
final class CoShellLogoView: UIView {

    private static let art: [String] = [
        "",
        "",
        "",
        "   #####",
        "  ########",
        " ##########",
        "## ## ## ###",
        "   ## ## #",
        " ##########",
        "  ########",
        "",
        "",
    ]

    /// The accent colour used for filled cells.
    var cellColor: UIColor = .systemTeal {
        didSet { setNeedsDisplay() }
    }

    /// A sensible default size so the view lays out correctly inside a
    /// UIStackView (which has no intrinsic size of its own). Callers that need
    /// a different size pin explicit width/height constraints instead.
    override var intrinsicContentSize: CGSize {
        CGSize(width: 33, height: 33)
    }

    override init(frame: CGRect) {
        super.init(frame: frame)
        backgroundColor = .clear
        isOpaque = false
    }

    required init?(coder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }

    override func draw(_ rect: CGRect) {
        let rows = Self.art.count
        let cols = Self.art.map { $0.count }.max() ?? 0
        guard rows > 0, cols > 0 else { return }

        // Square cells sized to fit the view while keeping the aspect ratio.
        let cell = min(bounds.width / CGFloat(cols), bounds.height / CGFloat(rows))
        let totalW = cell * CGFloat(cols)
        let totalH = cell * CGFloat(rows)
        let originX = (bounds.width - totalW) / 2
        let originY = (bounds.height - totalH) / 2

        cellColor.setFill()
        for (r, line) in Self.art.enumerated() {
            for (c, ch) in line.enumerated() where ch == "#" {
                let cellRect = CGRect(
                    x: originX + CGFloat(c) * cell,
                    y: originY + CGFloat(r) * cell,
                    width: cell,
                    height: cell
                )
                UIRectFill(cellRect)
            }
        }
    }
}
