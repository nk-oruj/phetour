<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    <xsl:output method="xml" omit-xml-declaration="yes"/>
    <xsl:strip-space elements="*"/>
    <xsl:param name="member-limit"/>

    <xsl:template match="/library-update">
        <rss-content>
            <title>Library update</title>
            <description><xsl:apply-templates select="created|revised|deleted"/></description>
        </rss-content>
    </xsl:template>

    <xsl:template match="created">
        <p><strong>Created</strong></p>
        <ul><xsl:apply-templates select="post|tag" mode="current-page"/></ul>
    </xsl:template>

    <xsl:template match="revised">
        <p><strong>Revised</strong></p>
        <ul><xsl:apply-templates select="post|tag" mode="current-page"/></ul>
    </xsl:template>

    <xsl:template match="deleted">
        <p><strong>Deleted</strong></p>
        <ul><xsl:apply-templates select="post|tag" mode="deleted-page"/></ul>
    </xsl:template>

    <xsl:template match="post|tag" mode="current-page">
        <li>
            <a href="{concat(/library-update/@site-url, '/', @id, '/')}">[<xsl:value-of select="@id"/>] - <xsl:value-of select="@title"/></a>
            <xsl:if test="member">
                <ul>
                    <li><strong><xsl:choose><xsl:when test="self::post">Tags</xsl:when><xsl:otherwise>Posts</xsl:otherwise></xsl:choose></strong></li>
                    <xsl:apply-templates select="member[position() &lt;= number($member-limit)]" mode="member"/>
                    <xsl:if test="count(member) &gt; number($member-limit)"><li>…</li></xsl:if>
                </ul>
            </xsl:if>
        </li>
    </xsl:template>

    <xsl:template match="post|tag" mode="deleted-page">
        <li>[<xsl:value-of select="@id"/>] - <xsl:value-of select="@title"/></li>
    </xsl:template>

    <xsl:template match="member" mode="member">
        <li><a href="{concat(/library-update/@site-url, '/', @id, '/')}">[<xsl:value-of select="@id"/>] - <xsl:value-of select="@title"/></a></li>
    </xsl:template>
</xsl:stylesheet>
